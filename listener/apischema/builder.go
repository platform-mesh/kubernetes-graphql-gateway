package apischema

import (
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/openmfp/kubernetes-graphql-gateway/common"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	runtimeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// SchemaBuilder helps construct GraphQL field config arguments
type SchemaBuilder struct {
	schemas map[string]*spec.Schema
	err     *multierror.Error
}

func NewSchemaBuilder(oc openapi.Client, preferredApiGroups []string) *SchemaBuilder {
	b := &SchemaBuilder{
		schemas: make(map[string]*spec.Schema),
	}

	apiv3Paths, err := oc.Paths()
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("failed to get OpenAPI paths: %w", err))
		return b
	}

	for path, gv := range apiv3Paths {
		schema, err := getSchemaForPath(preferredApiGroups, path, gv)
		if err != nil {
			//TODO: debug log?
			continue
		}
		maps.Copy(b.schemas, schema)
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
		if schema.VendorExtensible.Extensions == nil {
			continue
		}
		var gvksVal any
		var ok bool
		if gvksVal, ok = schema.VendorExtensible.Extensions[common.GVKExtensionKey]; !ok {
			continue
		}
		b, err := json.Marshal(gvksVal)
		if err != nil {
			//TODO: debug log?
			continue
		}
		gvks := make([]*GroupVersionKind, 0, 1)
		if err := json.Unmarshal(b, &gvks); err != nil {
			//TODO: debug log?
			continue
		}

		if len(gvks) != 1 {
			//TODO: debug log?
			continue
		}

		namespaced, err := apiutil.IsGVKNamespaced(k8sschema.GroupVersionKind{
			Group:   gvks[0].Group,
			Version: gvks[0].Version,
			Kind:    gvks[0].Kind,
		}, rm)

		if err != nil {
			//TODO: debug log?
			continue
		}

		if namespaced {
			schema.VendorExtensible.AddExtension(common.ScopeExtensionKey, apiextensionsv1.NamespaceScoped)
		} else {
			schema.VendorExtensible.AddExtension(common.ScopeExtensionKey, apiextensionsv1.ClusterScoped)
		}

	}

	return b
}

func (b *SchemaBuilder) WithCRDCategories(crd *apiextensionsv1.CustomResourceDefinition) *SchemaBuilder {
	categories := crd.Spec.Names.Categories
	if len(categories) == 0 {
		return b
	}
	gvk, err := getCRDGroupVersionKind(crd.Spec)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("failed to get CRD GVK: %w", err))
		return b
	}

	schema, ok := b.schemas[getOpenAPISchemaKey(*gvk)]
	if !ok {
		return b
	}

	schema.VendorExtensible.AddExtension(common.CategoriesExtensionKey, categories)

	return b
}

func (b *SchemaBuilder) WithApiResourceCategories(list []*metav1.APIResourceList) *SchemaBuilder {
	for _, apiResourceList := range list {
		for _, apiResource := range apiResourceList.APIResources {
			if apiResource.Categories == nil {
				continue
			}

			gv, err := runtimeSchema.ParseGroupVersion(apiResourceList.GroupVersion)
			if err != nil {
				b.err = multierror.Append(b.err, fmt.Errorf("failed to parse groupVersion: %w", err))
				continue
			}
			gvk := metav1.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    apiResource.Kind,
			}

			schema, ok := b.schemas[getOpenAPISchemaKey(gvk)]
			if !ok {
				continue
			}

			schema.VendorExtensible.AddExtension(common.CategoriesExtensionKey, apiResource.Categories)
		}
	}

	return b
}

func (b *SchemaBuilder) Complete() ([]byte, error) {
	v3JSON, err := json.Marshal(&schemaResponse{
		Components: schemasComponentsWrapper{
			Schemas: b.schemas,
		},
	})
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("failed to marshal openAPI v3 runtimeSchema: %w", err))
		return nil, b.err
	}
	v2JSON, err := ConvertJSON(v3JSON)
	if err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("failed to convert openAPI v3 runtimeSchema to v2: %w", err))
		return nil, b.err
	}

	return v2JSON, nil
}

func getOpenAPISchemaKey(gvk metav1.GroupVersionKind) string {
	// we need to inverse group to match the runtimeSchema key(io.openmfp.core.v1alpha1.Account)
	parts := strings.Split(gvk.Group, ".")
	slices.Reverse(parts)
	reversedGroup := strings.Join(parts, ".")

	return fmt.Sprintf("%s.%s.%s", reversedGroup, gvk.Version, gvk.Kind)
}

func getCRDGroupVersionKind(spec apiextensionsv1.CustomResourceDefinitionSpec) (*metav1.GroupVersionKind, error) {
	if len(spec.Versions) == 0 {
		return nil, fmt.Errorf("CRD has no versions defined")
	}

	// Use the first stored version as the preferred one
	return &metav1.GroupVersionKind{
		Group:   spec.Group,
		Version: spec.Versions[0].Name,
		Kind:    spec.Names.Kind,
	}, nil
}
