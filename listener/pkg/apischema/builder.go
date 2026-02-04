package apischema

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"github.com/platform-mesh/kubernetes-graphql-gateway/apis"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/schemamutation"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	ErrGetOpenAPIPaths      = errors.New("failed to get OpenAPI paths")
	ErrGetCRDGVK            = errors.New("failed to get CRD GVK")
	ErrParseGroupVersion    = errors.New("failed to parse groupVersion")
	ErrMarshalOpenAPISchema = errors.New("failed to marshal openAPI v3 runtimeSchema")
	ErrConvertOpenAPISchema = errors.New("failed to convert openAPI v3 runtimeSchema to v2")
	ErrCRDNoVersions        = errors.New("CRD has no versions defined")
	ErrMarshalGVK           = errors.New("failed to marshal GVK extension")
	ErrUnmarshalGVK         = errors.New("failed to unmarshal GVK extension")
	ErrBuildKindRegistry    = errors.New("failed to build kind registry")
)

type SchemaBuilder struct {
	schemas           map[string]*spec.Schema
	err               *multierror.Error
	log               logr.Logger
	kindRegistry      map[GroupVersionKind]ResourceInfo // Changed: Use GVK as key for precise lookup
	preferredVersions map[string]string                 // map[group/kind]preferredVersion
}

// ResourceInfo holds information about a resource for relationship resolution
type ResourceInfo struct {
	Group     string
	Version   string
	Kind      string
	SchemaKey string
}

func NewSchemaBuilder(oc openapi.Client, preferredApiGroups []string, log logr.Logger) *SchemaBuilder {
	b := &SchemaBuilder{
		schemas:           make(map[string]*spec.Schema),
		kindRegistry:      make(map[GroupVersionKind]ResourceInfo),
		preferredVersions: make(map[string]string),
		log:               log,
	}

	apiv3Paths, err := oc.Paths()
	if err != nil {
		b.err = multierror.Append(b.err, errors.Join(ErrGetOpenAPIPaths, err))
		return b
	}

	walker := schemamutation.Walker{
		RefCallback: schemamutation.RefCallbackNoop,
		SchemaCallback: func(schema *spec.Schema) *spec.Schema {
			refPtr := schema.Ref.GetPointer()
			if refPtr == nil {
				return schema
			}

			tokens := refPtr.DecodedTokens()
			if len(tokens) == 0 {
				return schema
			}

			resolvedRef := tokens[len(tokens)-1]

			return &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Ref: spec.MustCreateRef(resolvedRef),
				},
			}
		},
	}

	for _, path := range apiv3Paths {

		schemaBytes, err := path.Schema(discovery.AcceptV2)
		if err != nil {
			continue
		}

		var openAPISpec spec3.OpenAPI
		if err := json.Unmarshal(schemaBytes, &openAPISpec); err != nil {
			continue
		}

		if openAPISpec.Components == nil {
			continue
		}

		for k, v := range openAPISpec.Components.Schemas {
			out := walker.WalkSchema(v)
			b.schemas[k] = out
		}
	}

	return b
}

type GroupVersionKind struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

func (b *SchemaBuilder) WithScope(rm meta.RESTMapper) *SchemaBuilder {
	for _, schema := range b.schemas {
		//skip resources that do not have the GVK extension:
		//assumption: sub-resources do not have GVKs
		if schema.Extensions == nil {
			continue
		}
		var gvksVal any
		var ok bool
		if gvksVal, ok = schema.Extensions[apis.GVKExtensionKey]; !ok {
			continue
		}
		jsonBytes, err := json.Marshal(gvksVal)
		if err != nil {
			b.err = multierror.Append(b.err, errors.Join(ErrMarshalGVK, err))
			continue
		}
		gvks := make([]*GroupVersionKind, 0, 1)
		if err := json.Unmarshal(jsonBytes, &gvks); err != nil {
			b.err = multierror.Append(b.err, errors.Join(ErrUnmarshalGVK, err))
			continue
		}

		if len(gvks) != 1 {
			b.log.V(4).Info("skipping schema with unexpected GVK count", "gvkCount", len(gvks))
			continue
		}

		namespaced, err := apiutil.IsGVKNamespaced(runtimeSchema.GroupVersionKind{
			Group:   gvks[0].Group,
			Version: gvks[0].Version,
			Kind:    gvks[0].Kind,
		}, rm)

		if err != nil {
			b.log.V(4).Info("failed to get namespaced info for GVK",
				"error", err,
				"group", gvks[0].Group,
				"version", gvks[0].Version,
				"kind", gvks[0].Kind)
			continue
		}

		if namespaced {
			schema.AddExtension(apis.ScopeExtensionKey, apiextensionsv1.NamespaceScoped)
		} else {
			schema.AddExtension(apis.ScopeExtensionKey, apiextensionsv1.ClusterScoped)
		}
	}
	return b
}

func (b *SchemaBuilder) WithApiResourceCategories(list []*metav1.APIResourceList) *SchemaBuilder {
	if len(list) == 0 {
		return b
	}

	for _, apiResourceList := range list {
		gv, err := runtimeSchema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			b.err = multierror.Append(b.err, errors.Join(ErrParseGroupVersion, err))
			continue
		}
		for _, apiResource := range apiResourceList.APIResources {
			if apiResource.Categories == nil {
				continue
			}
			gvk := metav1.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: apiResource.Kind}
			resourceKey := getOpenAPISchemaKey(gvk)
			resourceSchema, ok := b.schemas[resourceKey]
			if !ok {
				continue
			}
			resourceSchema.AddExtension(apis.CategoriesExtensionKey, apiResource.Categories)
			b.schemas[resourceKey] = resourceSchema
		}
	}
	return b
}

// WithPreferredVersions populates preferred version information from API discovery
func (b *SchemaBuilder) WithPreferredVersions(apiResLists []*metav1.APIResourceList) *SchemaBuilder {
	for _, apiResList := range apiResLists {
		gv, err := runtimeSchema.ParseGroupVersion(apiResList.GroupVersion)
		if err != nil {
			b.log.V(4).Info("failed to parse group version",
				"error", err,
				"groupVersion", apiResList.GroupVersion)
			continue
		}

		for _, resource := range apiResList.APIResources {
			// Create a key for group/kind to track preferred version
			key := fmt.Sprintf("%s/%s", gv.Group, resource.Kind)

			// Store this version as preferred for this group/kind
			// ServerPreferredResources returns the preferred version for each group
			b.preferredVersions[key] = gv.Version

			b.log.V(5).Info("registered preferred version",
				"group", gv.Group,
				"kind", resource.Kind,
				"preferredVersion", gv.Version)
		}
	}
	return b
}

// WithRelationships adds relationship fields to schemas that have *Ref fields
// Uses 1-level depth control to prevent circular references and N+1 problems
func (b *SchemaBuilder) WithRelationships() *SchemaBuilder {
	// Build kind registry first
	b.buildKindRegistry()

	// Expand relationships with 1-level depth control
	b.expandWithSimpleDepthControl()

	return b
}

// expandWithSimpleDepthControl implements the working 1-level depth control
func (b *SchemaBuilder) expandWithSimpleDepthControl() {
	// First pass: identify relation targets
	relationTargets := make(map[string]bool)
	for _, schema := range b.schemas {
		if schema.Properties == nil {
			continue
		}
		for propName := range schema.Properties {
			if !isRefProperty(propName) {
				continue
			}
			baseKind := strings.TrimSuffix(propName, "Ref")
			candidates := b.findAllCandidatesForKind(baseKind)

			// Mark all candidates as relation targets
			for _, candidate := range candidates {
				relationTargets[candidate.SchemaKey] = true
			}
		}
	}

	b.log.V(2).Info("starting 1-level relationship expansion",
		"kindRegistrySize", len(b.kindRegistry),
		"relationTargets", len(relationTargets))

	// Second pass: expand only non-targets
	for schemaKey, schema := range b.schemas {
		if relationTargets[schemaKey] {
			b.log.V(4).Info("skipping relation target (1-level depth control)", "schemaKey", schemaKey)
			continue
		}
		b.expandRelationshipsSimple(schema, schemaKey)
	}
}

// buildKindRegistry builds a map of kind names to available resource types
func (b *SchemaBuilder) buildKindRegistry() {
	for schemaKey, schema := range b.schemas {
		// Extract GVK from schema
		if schema.Extensions == nil {
			continue
		}

		gvksVal, ok := schema.Extensions[apis.GVKExtensionKey]
		if !ok {
			continue
		}

		jsonBytes, err := json.Marshal(gvksVal)
		if err != nil {
			b.err = multierror.Append(b.err, errors.Join(ErrBuildKindRegistry, err))
			b.log.V(4).Info("failed to marshal GVK", "error", err, "schemaKey", schemaKey)
			continue
		}

		var gvks []*GroupVersionKind
		if err := json.Unmarshal(jsonBytes, &gvks); err != nil {
			b.err = multierror.Append(b.err, errors.Join(ErrBuildKindRegistry, err))
			b.log.V(4).Info("failed to unmarshal GVK", "error", err, "schemaKey", schemaKey)
			continue
		}

		if len(gvks) != 1 {
			continue
		}

		gvk := gvks[0]

		// Add to kind registry with precise GVK key
		resourceInfo := ResourceInfo{
			Group:     gvk.Group,
			Version:   gvk.Version,
			Kind:      gvk.Kind,
			SchemaKey: schemaKey,
		}

		// Index by full GroupVersionKind for precise lookup (no collisions)
		gvkKey := GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		}
		b.kindRegistry[gvkKey] = resourceInfo

	}

	// No sorting needed - each GVK is now uniquely indexed
	// Check for kinds with multiple resources but no preferred versions
	b.warnAboutMissingPreferredVersions()

	b.log.V(4).Info("built kind registry for relationships", "gvkCount", len(b.kindRegistry))
}

// warnAboutMissingPreferredVersions checks for kinds with multiple resources but no preferred versions
func (b *SchemaBuilder) warnAboutMissingPreferredVersions() {
	// Group resources by kind name to find potential conflicts
	kindGroups := make(map[string][]ResourceInfo)

	for _, resourceInfo := range b.kindRegistry {
		kindKey := strings.ToLower(resourceInfo.Kind)
		kindGroups[kindKey] = append(kindGroups[kindKey], resourceInfo)
	}

	// Check each kind that has multiple resources
	for kindName, resources := range kindGroups {
		if len(resources) <= 1 {
			continue // No conflict possible
		}

		// Check if any of the resources has a preferred version
		hasPreferred := false
		for _, resource := range resources {
			key := fmt.Sprintf("%s/%s", resource.Group, resource.Kind)
			if b.preferredVersions[key] == resource.Version {
				hasPreferred = true
				break
			}
		}

		// Warn if no preferred version found
		if !hasPreferred {
			groups := make([]string, 0, len(resources))
			for _, resource := range resources {
				groups = append(groups, fmt.Sprintf("%s/%s", resource.Group, resource.Version))
			}
			b.log.Info("multiple resources found for kind with no preferred version - using fallback resolution",
				"kind", kindName,
				"availableResources", groups)
		}
	}
}

// expandRelationshipsSimple adds relationship fields for the simple 1-level depth control
func (b *SchemaBuilder) expandRelationshipsSimple(schema *spec.Schema, schemaKey string) {
	if schema.Properties == nil {
		return
	}

	for propName := range schema.Properties {
		if !isRefProperty(propName) {
			continue
		}

		baseKind := strings.TrimSuffix(propName, "Ref")

		// Add relationship field using kubectl-style priority resolution
		b.processReferenceField(schema, schemaKey, propName, baseKind)
	}
}

// processReferenceField handles individual reference field processing with kubectl-style priority resolution
func (b *SchemaBuilder) processReferenceField(schema *spec.Schema, schemaKey, propName, baseKind string) {
	// Find best resource using kubectl-style priority
	bestResource := b.findBestResourceForKind(baseKind)

	if bestResource == nil {
		// No candidates found - skip relationship field generation
		b.log.V(4).Info("no candidates found for kind - skipping relationship field",
			"kind", baseKind,
			"sourceField", propName,
			"sourceSchema", schemaKey)
		return
	}

	// Generate relationship field using the best resource
	b.addRelationshipField(schema, schemaKey, propName, baseKind, bestResource)
}

// findBestResourceForKind finds the best resource for a kind using kubectl-style priority resolution
func (b *SchemaBuilder) findBestResourceForKind(kindName string) *ResourceInfo {
	candidates := b.findAllCandidatesForKind(kindName)

	if len(candidates) == 0 {
		return nil
	}

	if len(candidates) == 1 {
		return &candidates[0]
	}

	// Multiple candidates - use kubectl-style priority resolution
	best := b.selectByKubectlPriority(candidates)

	// Log warning about the conflict for observability
	groups := make([]string, len(candidates))
	for i, candidate := range candidates {
		groups[i] = b.formatGroupVersion(candidate)
	}
	b.log.Info("multiple API groups provide this kind - selected first by priority (kubectl-style)",
		"kind", kindName,
		"selectedGroup", b.formatGroupVersion(best),
		"availableGroups", groups)

	return &best
}

// findAllCandidatesForKind finds all resources that match the given kind name
func (b *SchemaBuilder) findAllCandidatesForKind(kindName string) []ResourceInfo {
	candidates := make([]ResourceInfo, 0)

	for gvk, resourceInfo := range b.kindRegistry {
		if strings.EqualFold(gvk.Kind, kindName) {
			candidates = append(candidates, resourceInfo)
		}
	}

	return candidates
}

// selectByKubectlPriority selects the best resource using kubectl's priority rules
func (sb *SchemaBuilder) selectByKubectlPriority(candidates []ResourceInfo) ResourceInfo {
	// Sort candidates by kubectl priority:
	// 1. Preferred versions first
	// 2. Core groups (empty group) over extensions
	// 3. Alphabetical by group name
	// 4. Alphabetical by version (newer versions typically sort later)
	slices.SortFunc(candidates, func(a, b ResourceInfo) int {
		// 1. Check preferred versions first
		aPreferred := sb.isPreferredVersion(a)
		bPreferred := sb.isPreferredVersion(b)
		if aPreferred && !bPreferred {
			return -1 // a wins
		}
		if !aPreferred && bPreferred {
			return 1 // b wins
		}

		// 2. Core groups (empty group) beat extension groups
		aCoreGroup := (a.Group == "")
		bCoreGroup := (b.Group == "")
		if aCoreGroup && !bCoreGroup {
			return -1 // a wins (core group)
		}
		if !aCoreGroup && bCoreGroup {
			return 1 // b wins (core group)
		}

		// 3. Alphabetical by group name
		if cmp := strings.Compare(a.Group, b.Group); cmp != 0 {
			return cmp
		}

		// 4. Alphabetical by version (this gives deterministic results)
		return strings.Compare(a.Version, b.Version)
	})

	return candidates[0] // Return the first (highest priority) candidate
}

// isPreferredVersion checks if this resource version is marked as preferred
func (b *SchemaBuilder) isPreferredVersion(resource ResourceInfo) bool {
	key := fmt.Sprintf("%s/%s", resource.Group, resource.Kind)
	return b.preferredVersions[key] == resource.Version
}

// formatGroupVersion formats a resource for display
func (b *SchemaBuilder) formatGroupVersion(resource ResourceInfo) string {
	if resource.Group == "" {
		return fmt.Sprintf("core/%s", resource.Version)
	}
	return fmt.Sprintf("%s/%s", resource.Group, resource.Version)
}

// addRelationshipField adds a relationship field for unambiguous references
func (b *SchemaBuilder) addRelationshipField(schema *spec.Schema, schemaKey, propName, baseKind string, target *ResourceInfo) {
	fieldName := strings.ToLower(baseKind)
	if _, exists := schema.Properties[fieldName]; exists {
		return
	}

	// Create proper reference - handle empty group (core) properly
	var refPath string
	if target.Group == "" {
		refPath = fmt.Sprintf("#/definitions/%s.%s", target.Version, target.Kind)
	} else {
		refPath = fmt.Sprintf("#/definitions/%s.%s.%s", target.Group, target.Version, target.Kind)
	}
	ref := spec.MustCreateRef(refPath)
	schema.Properties[fieldName] = spec.Schema{SchemaProps: spec.SchemaProps{Ref: ref}}

	b.log.V(3).Info("added relationship field",
		"sourceField", propName,
		"targetField", fieldName,
		"targetKind", target.Kind,
		"targetGroup", target.Group,
		"refPath", refPath,
		"sourceSchema", schemaKey)
}

func isRefProperty(name string) bool {
	if !strings.HasSuffix(name, "Ref") {
		return false
	}
	if name == "Ref" {
		return false
	}
	return true
}

func (b *SchemaBuilder) Complete() ([]byte, error) {
	v3JSON, err := json.Marshal(&spec3.OpenAPI{
		Components: &spec3.Components{
			Schemas: b.schemas,
		},
	})
	if err != nil {
		return nil, errors.Join(ErrMarshalOpenAPISchema, err)
	}

	return v3JSON, nil
}

// getOpenAPISchemaKey creates the key that kubernetes uses in its OpenAPI Definitions
func getOpenAPISchemaKey(gvk metav1.GroupVersionKind) string {
	// we need to inverse group to match the runtimeSchema key(io.openmfp.core.v1alpha1.Account)
	parts := strings.Split(gvk.Group, ".")
	slices.Reverse(parts)
	reversedGroup := strings.Join(parts, ".")

	return fmt.Sprintf("%s.%s.%s", reversedGroup, gvk.Version, gvk.Kind)
}
