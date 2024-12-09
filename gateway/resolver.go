package gateway

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/graphql-go/graphql"
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	authzv1 "k8s.io/api/authorization/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type resolver struct {
	conf *Config
}

func NewResolver(conf *Config) *resolver {
	return &resolver{conf: conf}
}

func (r *resolver) getListArguments() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"labelselector": &graphql.ArgumentConfig{
			Type:        graphql.String,
			Description: "a label selector to filter the objects by",
		},
		"namespace": &graphql.ArgumentConfig{
			Type:        graphql.String,
			Description: "the namespace in which to search for the objects",
		},
	}
}

func (r *resolver) listItems(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "list"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))

	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "Resolve", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		listFunc, ok := r.conf.pluralToListType[crd.Spec.Names.Plural]
		if !ok {
			logger.Error("no typed client available for the reuqested type")
			return nil, errors.New("no typed client available for the reuqested type")
		}

		list := listFunc()

		var opts []client.ListOption
		if labelSelector, ok := p.Args["labelselector"].(string); ok && labelSelector != "" {
			selector, err := labels.Parse(labelSelector)
			if err != nil {
				logger.Error("unable to parse given label selector", slog.Any("error", err))
				return nil, err
			}
			opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
		}

		if namespace, ok := p.Args["namespace"].(string); ok && namespace != "" {
			opts = append(opts, client.InNamespace(namespace))
		}

		err := r.conf.Client.List(ctx, list, opts...)
		if err != nil {
			logger.Error("unable to list objects", slog.Any("error", err))
			return nil, err
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			logger.Error("unable to extract list", slog.Any("error", err))
			return nil, err
		}

		// the controller-runtime cache returns unordered results so we sort it here
		slices.SortFunc(items, func(a runtime.Object, b runtime.Object) int {
			return strings.Compare(a.(client.Object).GetName(), b.(client.Object).GetName())
		})

		return items, nil
	}
}

func (r *resolver) getItemArguments() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"name": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "the metadata.name of the object",
		},
		"namespace": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "the metadata.namespace of the object",
		},
	}
}

func (r *resolver) getChangeArguments(input graphql.Input) graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"metadata": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(metadataInput),
			Description: "the metadata of the object",
		},
		"spec": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(input),
			Description: "the spec of the object",
		},
	}
}

func (r *resolver) getPatchArguments() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"type": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "The JSON patch type, it can be json-patch, merge-patch, strategic-merge-patch",
		},
		"payload": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "The JSON patch to apply to the object",
		},
		"metadata": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(metadataInput),
			Description: "Metadata including name and namespace of the object you want to patch",
		},
	}
}

func (r *resolver) getItem(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "get"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "Resolve", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		name, ok := p.Args["name"].(string)
		if !ok {
			return nil, errors.New("name key does not exist or is not a string")
		}

		namespace, ok := p.Args["namespace"].(string)
		if !ok {
			return nil, errors.New("namespace key does not exist or is not a string")
		}

		if err := isAuthorized(ctx, r.conf.Client, authzv1.ResourceAttributes{
			Verb:      "get",
			Group:     crd.Spec.Group,
			Version:   typeInformation.Name,
			Resource:  crd.Spec.Names.Plural,
			Namespace: namespace,
			Name:      name,
		}); err != nil {
			return nil, err
		}

		objectFunc, ok := r.conf.pluralToObjectType[crd.Spec.Names.Plural]
		if !ok {
			logger.Error("no typed client available for the reuqested type")
			return nil, errors.New("no typed client available for the reuqested type")
		}

		obj := objectFunc()
		err := r.conf.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj)
		if err != nil {
			logger.Error("unable to get object", slog.Any("error", err))
			return nil, err
		}

		return obj, nil
	}
}

func (r *resolver) deleteItem(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "delete"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "Delete", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		us := &unstructured.Unstructured{}
		us.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: typeInformation.Name,
			Kind:    crd.Spec.Names.Kind,
		})

		us.SetNamespace(p.Args["namespace"].(string))
		us.SetName(p.Args["name"].(string))

		err := r.conf.Client.Delete(ctx, us)
		if err != nil {
			logger.Error("unable to delete object", slog.Any("error", err))
			return false, err
		}

		return true, nil
	}
}

func (r *resolver) createItem(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "create"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "Create", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		var metadatInput MetadatInput
		if err := mapstructure.Decode(p.Args["metadata"], &metadatInput); err != nil {
			logger.Error("unable to decode metadata input", slog.Any("error", err))
			return nil, err
		}

		logger = logger.With(slog.Group("metadata", slog.String("name", metadatInput.Name), slog.String("namespace", metadatInput.Namespace)))

		us := &unstructured.Unstructured{}
		us.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: typeInformation.Name,
			Kind:    crd.Spec.Names.Kind,
		})

		us.SetNamespace(metadatInput.Namespace)
		if metadatInput.Name != "" {
			us.SetName(metadatInput.Name)
		}

		if metadatInput.GenerateName != "" {
			us.SetGenerateName(metadatInput.GenerateName)
		}

		if metadatInput.Labels != nil {
			us.SetLabels(metadatInput.Labels)
		}

		if us.GetName() == "" && us.GetGenerateName() == "" {
			logger.Error("either name or generateName must be set")
			return nil, errors.New("either name or generateName must be set")
		}

		err := unstructured.SetNestedField(us.Object, p.Args["spec"], "spec")
		if err != nil {
			logger.Error("unable to set spec field", slog.Any("error", err))
			return nil, err
		}

		err = r.conf.Client.Create(ctx, us)
		if err != nil {
			logger.Error("unable to create object", slog.Any("error", err))
			return nil, err
		}

		return us.Object, nil
	}
}

func (r *resolver) updateItem(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "update"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "Update", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		var metadatInput MetadatInput
		if err := mapstructure.Decode(p.Args["metadata"], &metadatInput); err != nil {
			logger.Error("unable to decode metadata input", "error", err)
			return nil, err
		}

		logger = logger.With(slog.Group("metadata", slog.String("name", metadatInput.Name), slog.String("namespace", metadatInput.Namespace)))

		us := &unstructured.Unstructured{}
		us.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: typeInformation.Name,
			Kind:    crd.Spec.Names.Kind,
		})

		us.SetNamespace(metadatInput.Namespace)
		us.SetName(metadatInput.Name)

		err := r.conf.Client.Get(ctx, client.ObjectKey{Namespace: us.GetNamespace(), Name: us.GetName()}, us)
		if err != nil {
			logger.Error("unable to get object", slog.Any("error", err))
			return nil, err
		}

		err = unstructured.SetNestedField(us.Object, p.Args["spec"], "spec")
		if err != nil {
			logger.Error("unable to set spec field", slog.Any("error", err))
			return nil, err
		}

		err = r.conf.Client.Update(ctx, us)
		if err != nil {
			logger.Error("unable to update object", slog.Any("error", err))
			return nil, err
		}

		return us.Object, nil
	}
}

func (r *resolver) patchItem(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "patch-json"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "Patch", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		var metadatInput MetadatInput
		if err := mapstructure.Decode(p.Args["metadata"], &metadatInput); err != nil {
			logger.Error("unable to decode metadata input", "error", err)
			return nil, err
		}

		logger = logger.With(slog.Group("metadata", slog.String("name", metadatInput.Name), slog.String("namespace", metadatInput.Namespace)))

		us := &unstructured.Unstructured{}
		us.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: typeInformation.Name,
			Kind:    crd.Spec.Names.Kind,
		})

		us.SetNamespace(metadatInput.Namespace)
		us.SetName(metadatInput.Name)

		err := r.conf.Client.Get(ctx, client.ObjectKey{Namespace: us.GetNamespace(), Name: us.GetName()}, us)
		if err != nil {
			logger.Error("unable to get object", slog.Any("error", err))
			return nil, err
		}

		payload, ok := p.Args["payload"].(string)
		if !ok {
			logger.Error("unable to parse payload field")
			return nil, errors.New("unable to parse payload field")
		}

		patchTypeArg, ok := p.Args["type"].(string)
		if !ok {
			logger.Error("unable to parse patch type field")
			return nil, errors.New("unable to parse patch type field")
		}

		var patchType types.PatchType
		switch patchTypeArg {
		case "json-patch":
			patchType = types.JSONPatchType
		case "merge-patch":
			patchType = types.MergePatchType
		case "strategic-merge-patch":
			patchType = types.StrategicMergePatchType
		default:
			logger.Error("invalid patch type, patch type can be json-patch, merge-patch or strategic-merge-patch")
			return nil, errors.New("invalid patch type, patch type can be json-patch, merge-patch or strategic-merge-patch")
		}

		patch := client.RawPatch(patchType, []byte(payload))

		err = r.conf.Client.Patch(ctx, us, patch, &client.PatchOptions{})

		if err != nil {
			logger.Error("unable to patch object", slog.Any("error", err))
			return nil, err
		}

		return us.Object, nil
	}
}

func (r *resolver) subscribeItem(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	logger := slog.With(slog.String("operation", "subribeItem"), slog.String("kind", crd.Spec.Names.Kind), slog.String("version", typeInformation.Name))
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "SubscribeForObject", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		var metadatInput MetadatInput
		if err := mapstructure.Decode(p.Args, &metadatInput); err != nil {
			logger.Error("unable to decode metadata input", "error", err)
			return nil, err
		}

		listType, ok := r.conf.pluralToListType[crd.Spec.Names.Plural]
		if !ok {
			logger.Error("no typed client available for the reuqested type")
			return nil, errors.New("no typed client available for the reuqested type")
		}

		if err := isAuthorized(ctx, r.conf.Client, authzv1.ResourceAttributes{
			Verb:      "watch",
			Group:     crd.Spec.Group,
			Version:   typeInformation.Name,
			Resource:  crd.Spec.Names.Plural,
			Namespace: metadatInput.Namespace,
			Name:      metadatInput.Name,
		}); err != nil {
			return nil, err
		}

		listWatch, err := r.conf.Client.Watch(ctx, listType(), client.InNamespace(metadatInput.Namespace), client.MatchingFields{"metadata.name": metadatInput.Name})
		if err != nil {
			logger.Error("unable to watch object", slog.Any("error", err))
			return nil, err
		}

		resultChannel := make(chan interface{})

		go func() {
			var item client.Object
			for ev := range listWatch.ResultChan() {
				changed := false
				select {
				case <-ctx.Done():
					slog.Info("stopping watch due to client cancel")
					listWatch.Stop()
					close(resultChannel)
					return
				default:
					switch ev.Type {
					case watch.Added:
						item = ev.Object.(client.Object)
						changed = true
					case watch.Modified:

						emitOnlyFieldChanges, ok := p.Args["emitOnlyFieldChanges"].(bool)

						changed = determineFieldChanged(ok && emitOnlyFieldChanges, ev.Object.(client.Object), resultChannel, p, item)
						item = ev.Object.(client.Object)
					case watch.Deleted:
						itemType, ok := r.conf.pluralToObjectType[crd.Spec.Names.Plural]
						if !ok {
							logger.Error("no typed client available for the reuqested type")
							listWatch.Stop()
							close(resultChannel)
						}

						item = itemType()
					default:
						logger.Info("skipping event", "event", ev.Type, "object", ev.Object)
						continue
					}
				}

				if val, ok := p.Args["emitOnlyFieldChanges"].(bool); ok && val && changed {
					resultChannel <- item
				} else if !ok || !val {
					resultChannel <- item
				}
			}
		}()

		return resultChannel, nil
	}
}

func (r *resolver) subscribeItems(crd apiextensionsv1.CustomResourceDefinition, typeInformation apiextensionsv1.CustomResourceDefinitionVersion) func(p graphql.ResolveParams) (interface{}, error) {
	return func(p graphql.ResolveParams) (interface{}, error) {
		ctx, span := otel.Tracer("").Start(p.Context, "SubscribeForNamespace", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
		defer span.End()

		listType, ok := r.conf.pluralToListType[crd.Spec.Names.Plural]
		if !ok {
			return nil, errors.New("no typed client available for the reuqested type")
		}

		if err := isAuthorized(ctx, r.conf.Client, authzv1.ResourceAttributes{
			Verb:      "watch",
			Group:     crd.Spec.Group,
			Version:   typeInformation.Name,
			Resource:  crd.Spec.Names.Plural,
			Namespace: p.Args["namespace"].(string),
		}); err != nil {
			return nil, err
		}

		listWatch, err := r.conf.Client.Watch(ctx, listType(), client.InNamespace(p.Args["namespace"].(string)))
		if err != nil {
			return nil, err
		}

		resultChannel := make(chan interface{})
		go func() {
			// TODO: i would like to figure out if there is another way than to buffer all the items
			items := []client.Object{}

			// TODO: only publish a event if one of the subcribed fields has changed
			for ev := range listWatch.ResultChan() {
				changed := false
				select {
				case <-ctx.Done():
					slog.Info("stopping watch due to client cancel")
					listWatch.Stop()
					close(resultChannel)
				default:
					switch ev.Type {
					case watch.Added:
						items = append(items, ev.Object.(client.Object))
						changed = true
					case watch.Modified:
						for i, item := range items {
							if item.GetName() == ev.Object.(client.Object).GetName() {

								emitOnlyFieldChanges, ok := p.Args["emitOnlyFieldChanges"].(bool)

								changed = determineFieldChanged(ok && emitOnlyFieldChanges, ev.Object.(client.Object), resultChannel, p, item)

								items[i] = ev.Object.(client.Object)
								break
							}
						}
					case watch.Deleted:
						for i, item := range items {
							if item.GetName() == ev.Object.(client.Object).GetName() {
								items = append(items[:i], items[i+1:]...)
								changed = true
								break
							}
						}
					default:
						slog.Info("skipping event", "event", ev.Type, "object", ev.Object)
						continue
					}

					if val, ok := p.Args["emitOnlyFieldChanges"].(bool); ok && val && changed {
						resultChannel <- items
					} else if !ok || !val {
						resultChannel <- items
					}
				}
			}
		}()

		return resultChannel, nil
	}
}

func determineFieldChanged(emitOnlyFieldChanges bool, obj client.Object, resultChannel chan interface{}, p graphql.ResolveParams, currentItem client.Object) bool {
	if !emitOnlyFieldChanges {
		return true
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		// TODO: handle error
		close(resultChannel)
	}

	fields := getRequestedFields(p)

	currentItemUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(currentItem)
	if err != nil {
		// TODO: handle error
		close(resultChannel)
	}

	for _, field := range fields {
		fieldValue, found, err := unstructured.NestedFieldNoCopy(unstructuredObj, strings.Split(field, ".")...)
		if err != nil {
			// TODO: handle error
			slog.Error("unable to get field value", "error", err)
			close(resultChannel)
		}

		currentFieldValue, currentFound, err := unstructured.NestedFieldNoCopy(currentItemUnstructured, strings.Split(field, ".")...)
		if err != nil {
			// TODO: handle error
			slog.Error("unable to get field value", "error", err)
			close(resultChannel)
		}

		if !found || !currentFound {
			continue
		}
		if cmp.Equal(fieldValue, currentFieldValue) {
			continue
		}

		return true
	}

	return false
}

func isAuthorized(ctx context.Context, c client.Client, resourceAttributes authzv1.ResourceAttributes) error {
	ctx, span := otel.Tracer("").Start(ctx, "AuthorizationCheck")
	defer span.End()

	user, ok := GetUserFromContext(ctx)
	if !ok || user == "" {
		return errors.New("no user found in context")
	}

	sar := authzv1.SubjectAccessReview{
		Spec: authzv1.SubjectAccessReviewSpec{
			User:               user,
			ResourceAttributes: &resourceAttributes,
		},
	}

	groups, ok := GetGroupsFromContext(ctx)
	if ok {
		sar.Spec.Groups = groups
	}

	err := c.Create(ctx, &sar)
	if err != nil {
		return err
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "SAR result",
		slog.String("kind", resourceAttributes.Resource),
		slog.String("namespace", resourceAttributes.Namespace),
		slog.String("user", user),
		slog.Bool("allowed", sar.Status.Allowed),
		slog.String("verb", resourceAttributes.Verb),
	)

	if !sar.Status.Allowed {
		return errors.New("access denied")
	}

	return nil
}
