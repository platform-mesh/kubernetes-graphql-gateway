package resolver

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/graphql-go/graphql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Service struct {
	runtimeClient client.WithWatch
}

func New(runtimeClient client.WithWatch) *Service {
	return &Service{
		runtimeClient: runtimeClient,
	}
}

func (r *Service) ListItems(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		logger := log.FromContext(p.Context)
		ctx, span := otel.Tracer("").Start(p.Context, "ListItems", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()

		logger = logger.WithValues(
			"operation", "list",
			"group", gvk.Group,
			"version", gvk.Version,
			"kind", gvk.Kind,
		)

		// Create an unstructured list to hold the results
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)

		var opts []client.ListOption

		labelSelector, err := GetArg[string](p.Args, LabelSelectorArg, false)
		if err != nil {
			return nil, err
		}
		if labelSelector != "" {
			selector, err := labels.Parse(labelSelector)
			if err != nil {
				logger.WithValues(LabelSelectorArg, labelSelector).Error(err, "Unable to parse given label selector")
				return nil, err
			}
			opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
		}

		if isResourceNamespaceScoped(scope) {
			namespace, err := GetArg[string](p.Args, NamespaceArg, false)
			if err != nil {
				return nil, err
			}
			if namespace != "" {
				opts = append(opts, client.InNamespace(namespace))
			}
		}

		limit, err := GetArg[int](p.Args, LimitArg, false)
		if err != nil {
			return nil, err
		}
		if limit > 0 {
			opts = append(opts, client.Limit(int64(limit)))
		}

		continueToken, err := GetArg[string](p.Args, ContinueArg, false)
		if err != nil {
			return nil, err
		}
		if continueToken != "" {
			opts = append(opts, client.Continue(continueToken))
		}

		if err = r.runtimeClient.List(ctx, list, opts...); err != nil {
			logger.Error(err, "Unable to list objects")
			return nil, fmt.Errorf("unable to list objects: %w", err)
		}

		sortBy, err := GetArg[string](p.Args, SortByArg, false)
		if err != nil {
			return nil, err
		}

		if sortBy != "" {
			if err := validateSortBy(list.Items, sortBy); err != nil {
				logger.WithValues(SortByArg, sortBy).Error(err, "Invalid sortBy field path")
				return nil, err
			}
			slices.SortFunc(list.Items, compareUnstructured(sortBy))
		}

		items := make([]map[string]any, len(list.Items))
		for i, item := range list.Items {
			items[i] = item.Object
		}

		return &ListResult{
			ResourceVersion:    list.GetResourceVersion(),
			Items:              items,
			Continue:           list.GetContinue(),
			RemainingItemCount: list.GetRemainingItemCount(),
		}, nil
	}
}

func (r *Service) GetItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		logger := log.FromContext(p.Context)
		ctx, span := otel.Tracer("").Start(p.Context, "GetItem", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()

		logger = logger.WithValues(
			"operation", "get",
			"group", gvk.Group,
			"version", gvk.Version,
			"kind", gvk.Kind,
		)

		// Retrieve required arguments
		name, err := GetArg[string](p.Args, NameArg, true)
		if err != nil {
			return nil, err
		}

		// Create an unstructured object to hold the result
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		key := client.ObjectKey{
			Name: name,
		}

		if isResourceNamespaceScoped(scope) {
			namespace, err := GetArg[string](p.Args, NamespaceArg, true)
			if err != nil {
				return nil, err
			}

			key.Namespace = namespace
		}

		// Get the object using the runtime client
		if err = r.runtimeClient.Get(ctx, key, obj); err != nil {
			logger.WithValues("name", name, "scope", string(scope)).Error(err, "Unable to get object")
			return nil, err
		}

		return obj.Object, nil
	}
}

func (r *Service) GetItemAsYAML(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		_, span := otel.Tracer("").Start(p.Context, "GetItemAsYAML", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()

		out, err := r.GetItem(gvk, scope)(p)
		if err != nil {
			return "", err
		}

		var returnYaml bytes.Buffer
		if err = yaml.NewEncoder(&returnYaml).Encode(out); err != nil {
			return "", err
		}

		return returnYaml.String(), nil
	}
}

func (r *Service) CreateItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "CreateItem", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()

		logger := log.FromContext(p.Context).WithValues(
			"operation", "create",
			"group", gvk.Group,
			"version", gvk.Version,
			"kind", gvk.Kind,
		)

		objectInput := p.Args["object"].(map[string]any)

		obj := &unstructured.Unstructured{
			Object: objectInput,
		}
		obj.SetGroupVersionKind(gvk)

		if isResourceNamespaceScoped(scope) {
			namespace, err := GetArg[string](p.Args, NamespaceArg, true)
			if err != nil {
				return nil, err
			}
			obj.SetNamespace(namespace)
		}

		if obj.GetName() == "" {
			return nil, errors.New("object metadata.name is required")
		}

		dryRunBool, err := GetArg[bool](p.Args, DryRunArg, false)
		if err != nil {
			return nil, err
		}
		dryRun := []string{}
		if dryRunBool {
			dryRun = []string{"All"}
		}

		if err := r.runtimeClient.Create(ctx, obj, &client.CreateOptions{DryRun: dryRun}); err != nil {
			logger.Error(err, "Failed to create object")
			return nil, err
		}

		return obj.Object, nil
	}
}

func (r *Service) UpdateItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		logger := log.FromContext(p.Context)
		ctx, span := otel.Tracer("").Start(p.Context, "UpdateItem", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()

		logger = logger.WithValues("operation", "update", "kind", gvk.Kind)

		name, err := GetArg[string](p.Args, NameArg, true)
		if err != nil {
			return nil, err
		}

		objectInput := p.Args[ObjectArg].(map[string]any)
		patchData, err := json.Marshal(objectInput)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal object input: %w", err)
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		obj.SetName(name)

		if isResourceNamespaceScoped(scope) {
			namespace, err := GetArg[string](p.Args, NamespaceArg, true)
			if err != nil {
				return nil, err
			}
			obj.SetNamespace(namespace)
		}

		dryRunBool, err := GetArg[bool](p.Args, DryRunArg, false)
		if err != nil {
			return nil, err
		}
		var dryRun []string
		if dryRunBool {
			dryRun = []string{"All"}
		}

		patch := client.RawPatch(types.MergePatchType, patchData)
		if err := r.runtimeClient.Patch(ctx, obj, patch, &client.PatchOptions{DryRun: dryRun}); err != nil {
			logger.Error(err, "Failed to patch object")
			return nil, err
		}

		return obj.Object, nil
	}
}

func (r *Service) DeleteItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		logger := log.FromContext(p.Context)
		ctx, span := otel.Tracer("").Start(p.Context, "DeleteItem", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()

		logger = logger.WithValues("operation", "delete", "kind", gvk.Kind)

		name, err := GetArg[string](p.Args, NameArg, true)
		if err != nil {
			return nil, err
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		obj.SetName(name)

		if isResourceNamespaceScoped(scope) {
			namespace, err := GetArg[string](p.Args, NamespaceArg, true)
			if err != nil {
				return nil, err
			}
			obj.SetNamespace(namespace)
		}

		dryRunBool, err := GetArg[bool](p.Args, DryRunArg, false)
		if err != nil {
			return nil, err
		}
		dryRun := []string{}
		if dryRunBool {
			dryRun = []string{"All"}
		}

		if err := r.runtimeClient.Delete(ctx, obj, &client.DeleteOptions{DryRun: dryRun}); err != nil {
			logger.Error(err, "Failed to delete object")
			return nil, err
		}

		return true, nil
	}
}

// ApplyYaml returns a resolver that applies a single YAML document to the
// Kubernetes API server with create-or-update semantics: if the resource
// exists it is updated, otherwise it is created.
func (r *Service) ApplyYaml() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "ApplyYaml")
		defer span.End()

		logger := log.FromContext(ctx).WithValues("operation", "apply")

		yamlStr, err := GetArg[string](p.Args, YamlArg, true)
		if err != nil {
			return nil, err
		}

		parsed, err := parseAndValidateYAML(yamlStr)
		if err != nil {
			return nil, err
		}

		obj := &unstructured.Unstructured{Object: parsed}

		gvk := obj.GetObjectKind().GroupVersionKind()
		name := obj.GetName()
		namespace := obj.GetNamespace()

		span.SetAttributes(
			attribute.String("kind", gvk.Kind),
			attribute.String("name", name),
		)

		logger = logger.WithValues(
			"group", gvk.Group,
			"version", gvk.Version,
			"kind", gvk.Kind,
			"name", name,
			"namespace", namespace,
		)

		target := &unstructured.Unstructured{}
		target.SetGroupVersionKind(gvk)
		target.SetName(name)
		target.SetNamespace(namespace)

		if _, err := controllerutil.CreateOrUpdate(ctx, r.runtimeClient, target, func() error {
			// Preserve server-managed fields that CreateOrUpdate fetched
			rv := target.GetResourceVersion()
			uid := target.GetUID()
			target.Object = parsed
			target.SetResourceVersion(rv)
			target.SetUID(uid)
			return nil
		}); err != nil {
			logger.Error(err, "Failed to apply YAML")
			return nil, fmt.Errorf("failed to apply resource %s/%s: %w", gvk.Kind, name, err)
		}

		return target.Object, nil
	}
}

func (r *Service) CommonResolver() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		return p.Source, nil
	}
}

func compareUnstructured(fieldPath string) func(a, b unstructured.Unstructured) int {
	return func(a, b unstructured.Unstructured) int {
		segments := strings.Split(fieldPath, ".")

		aVal, foundA, errA := unstructured.NestedFieldNoCopy(a.Object, segments...)
		bVal, foundB, errB := unstructured.NestedFieldNoCopy(b.Object, segments...)
		if errA != nil || errB != nil || !foundA || !foundB {
			return 0
		}

		switch av := aVal.(type) {
		case string:
			return cmp.Compare(av, bVal.(string))
		case float64:
			return cmp.Compare(av, bVal.(float64))
		case int64:
			return cmp.Compare(av, bVal.(int64))
		case bool:
			// bool not in cmp.Ordered; false < true
			if av == bVal.(bool) {
				return 0
			} else if bVal.(bool) {
				return -1
			}
			return 1
		}

		return 0
	}
}

// parseAndValidateYAML decodes a YAML string, rejects multi-document input,
// and validates that apiVersion, kind, and metadata.name are present.
func parseAndValidateYAML(yamlStr string) (map[string]any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(yamlStr)))

	var parsed map[string]any
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("multi-document YAML is not supported; provide a single document")
	}

	apiVersion, ok := parsed["apiVersion"].(string)
	if !ok || apiVersion == "" {
		return nil, errors.New("apiVersion is required and must be a string")
	}

	kind, ok := parsed["kind"].(string)
	if !ok || kind == "" {
		return nil, errors.New("kind is required and must be a string")
	}

	metadata, ok := parsed["metadata"].(map[string]any)
	if !ok || metadata == nil {
		return nil, errors.New("metadata is required")
	}
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return nil, errors.New("metadata.name is required and must be a string")
	}

	return parsed, nil
}
