package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/handler"
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	authzv1 "k8s.io/api/authorization/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var stringMapScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "StringMap",
	Description: "A map of strings, Commonly used for metadata.labels and metadata.annotations.",
	Serialize:   func(value interface{}) interface{} { return value },
	ParseValue:  func(value interface{}) interface{} { return value },
	ParseLiteral: func(valueAST ast.Value) interface{} {
		out := map[string]string{}

		switch value := valueAST.(type) {
		case *ast.ObjectValue:
			for _, field := range value.Fields {
				out[field.Name.Value] = field.Value.GetValue().(string)
			}
		}
		return out
	},
})

var objectMeta = graphql.NewObject(graphql.ObjectConfig{
	Name: "Metadata",
	Fields: graphql.Fields{
		"name": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "the metadata.name of the object",
		},
		"namespace": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "the metadata.namespace of the object",
		},
		"labels": &graphql.Field{
			Type:        stringMapScalar,
			Description: "the metadata.labels of the object",
		},
		"annotations": &graphql.Field{
			Type:        stringMapScalar,
			Description: "the metadata.annotations of the object",
		},
	},
})

var metadataInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetadataInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "the metadata.name of the object you want to create",
		},
		"generateName": &graphql.InputObjectFieldConfig{
			Type:        graphql.String,
			Description: "the metadata.generateName of the object you want to create",
		},
		"namespace": &graphql.InputObjectFieldConfig{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "the metadata.namespace of the object you want to create",
		},
		"labels": &graphql.InputObjectFieldConfig{
			Type:        stringMapScalar,
			Description: "the metadata.labels of the object you want to create",
		},
	},
})

type MetadatInput struct {
	Name         string            `mapstructure:"name,omitempty"`
	GenerateName string            `mapstructure:"generateName,omitempty"`
	Namespace    string            `mapstructure:"namespace,omitempty"`
	Labels       map[string]string `mapstructure:"labels,omitempty"`
}

func gqlTypeForOpenAPIProperties(in map[string]apiextensionsv1.JSONSchemaProps, fields graphql.Fields, inputFields graphql.InputObjectConfigFieldMap, parentFieldName string, requiredKeys []string) (graphql.Fields, graphql.InputObjectConfigFieldMap) {
	for key, info := range in {
		typeKey := strings.ReplaceAll(key, "-", "")
		currentField := &graphql.Field{
			Name:        typeKey,
			Description: info.Description,
		}
		currentInputField := &graphql.InputObjectFieldConfig{
			Description: info.Description,
		}

		switch info.Type {
		case "string":
			currentField.Type = graphql.String
			currentInputField.Type = graphql.String
		case "boolean":
			currentField.Type = graphql.Boolean
			currentInputField.Type = graphql.Boolean
		case "integer":
			currentField.Type = graphql.Int
			currentInputField.Type = graphql.Int
		case "array":
			typeName := parentFieldName + cases.Title(language.English).String(key) + "Item"

			if info.Items.Schema.Properties != nil { // nested array object
				newFields, newInputFields := gqlTypeForOpenAPIProperties(info.Items.Schema.Properties, graphql.Fields{}, graphql.InputObjectConfigFieldMap{}, typeName, info.Items.Schema.Required)
				newType := graphql.NewObject(graphql.ObjectConfig{
					Name:   typeName,
					Fields: newFields,
				})
				newInputType := graphql.NewInputObject(graphql.InputObjectConfig{
					Name:   typeName + "Input",
					Fields: newInputFields,
				})
				if len(newFields) == 0 {
					slog.Info("skipping creation of subtype due to emtpy field configuration", "type", typeName)
					continue
				}

				currentField.Type = graphql.NewList(newType)
				currentInputField.Type = graphql.NewList(newInputType)
			} else { // primitive array
				switch info.Items.Schema.Type {
				case "string":
					currentField.Type = graphql.String
					currentInputField.Type = graphql.String
				case "boolean":
					currentField.Type = graphql.Boolean
					currentInputField.Type = graphql.Boolean
				case "integer":
					currentField.Type = graphql.Int
					currentInputField.Type = graphql.Int
				}

				currentField.Type = graphql.NewList(currentField.Type)
				currentInputField.Type = graphql.NewList(currentInputField.Type)
			}
		case "object":
			if info.Properties == nil {
				continue
			}
			typeName := cases.Title(language.English).String(parentFieldName) + cases.Title(language.English).String(key)
			newFields, newInputFields := gqlTypeForOpenAPIProperties(info.Properties, graphql.Fields{}, graphql.InputObjectConfigFieldMap{}, typeName, info.Required)
			if len(newFields) == 0 {
				slog.Info("skipping creation of subtype due to emtpy field configuration", "type", typeName)
				continue
			}

			newType := graphql.NewObject(graphql.ObjectConfig{
				Name:   parentFieldName + key,
				Fields: newFields,
			})
			newInputType := graphql.NewInputObject(graphql.InputObjectConfig{
				Name:   typeName + "Input",
				Fields: newInputFields,
			})

			currentField.Type = newType
			currentInputField.Type = newInputType
		default:
			continue
		}

		if slices.Contains(requiredKeys, key) {
			currentField.Type = graphql.NewNonNull(currentField.Type)
			currentInputField.Type = graphql.NewNonNull(currentInputField.Type)
		}

		fields[typeKey] = currentField
		inputFields[typeKey] = currentInputField
	}

	return fields, inputFields
}

type Config struct {
	Client client.WithWatch

	// optional client.Reader to use for initial crd retrieval
	Reader client.Reader

	pluralToListType   map[string]func() client.ObjectList
	pluralToObjectType map[string]func() client.Object
}

func getTypesAndCRDsFromScheme(schema *runtime.Scheme, crds []apiextensionsv1.CustomResourceDefinition) (map[string]func() client.ObjectList, map[string]func() client.Object, []apiextensionsv1.CustomResourceDefinition) {
	pluralToList := map[string]func() client.ObjectList{}
	pluralToObject := map[string]func() client.Object{}
	activeCRDs := []apiextensionsv1.CustomResourceDefinition{}

	listInterface := reflect.TypeOf((*client.ObjectList)(nil)).Elem()
	objectInterface := reflect.TypeOf((*client.Object)(nil)).Elem()

	for gvk, knownType := range schema.AllKnownTypes() {

		idx := slices.IndexFunc(crds, func(crd apiextensionsv1.CustomResourceDefinition) bool {
			return strings.Contains(gvk.Kind, crd.Spec.Names.Kind) && crd.Spec.Group == gvk.Group
		})
		if idx == -1 {
			continue
		}

		if reflect.PointerTo(knownType).Implements(objectInterface) && !reflect.PointerTo(knownType).Implements(listInterface) {
			pluralToObject[crds[idx].Spec.Names.Plural] = func() client.Object {
				return reflect.New(knownType).Interface().(client.Object)
			}
		}

		if !reflect.PointerTo(knownType).Implements(listInterface) {
			continue
		}

		pluralToList[crds[idx].Spec.Names.Plural] = func() client.ObjectList {
			return reflect.New(knownType).Interface().(client.ObjectList)
		}

		activeCRDs = append(activeCRDs, crds[idx])
	}

	return pluralToList, pluralToObject, activeCRDs
}

func crdsByGroup(crds []apiextensionsv1.CustomResourceDefinition) map[string][]apiextensionsv1.CustomResourceDefinition {
	byGroup := map[string][]apiextensionsv1.CustomResourceDefinition{}

	for _, crd := range crds {
		var groupNameBuilder strings.Builder
		for i, part := range strings.Split(crd.Spec.Group, ".") {
			if i == 0 {
				groupNameBuilder.WriteString(part)
				continue
			}
			piece := cases.Title(language.English).String(part)
			groupNameBuilder.WriteString(piece)
		}
		byGroup[groupNameBuilder.String()] = append(byGroup[groupNameBuilder.String()], crd)
	}

	return byGroup
}

func getRequestedFields(params graphql.ResolveParams) []string {
	fieldSet := make(map[string]struct{})
	var traverseSelections func([]*ast.Field, string)

	traverseSelections = func(fields []*ast.Field, parentPath string) {
		for _, currentField := range fields {
			for _, selection := range currentField.SelectionSet.Selections {
				field, ok := selection.(*ast.Field)
				if !ok || field == nil {
					continue
				}

				fieldPath := field.Name.Value
				if parentPath != "" {
					fieldPath = parentPath + "." + field.Name.Value
				}

				if field.SelectionSet != nil {
					traverseSelections([]*ast.Field{field}, fieldPath)
				} else {
					fieldSet[fieldPath] = struct{}{}
				}
			}
		}
	}

	traverseSelections(params.Info.FieldASTs, "")

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	return fields
}

func New(ctx context.Context, conf Config) (graphql.Schema, error) {

	if conf.Client == nil {
		return graphql.Schema{}, errors.New("client is required")
	}

	if conf.Reader == nil {
		conf.Reader = conf.Client
	}

	var crdsList apiextensionsv1.CustomResourceDefinitionList
	err := conf.Reader.List(ctx, &crdsList)
	if err != nil {
		return graphql.Schema{}, err
	}

	var crds []apiextensionsv1.CustomResourceDefinition
	conf.pluralToListType, conf.pluralToObjectType, crds = getTypesAndCRDsFromScheme(conf.Client.Scheme(), crdsList.Items)

	rootQueryFields := graphql.Fields{}
	rootMutationFields := graphql.Fields{}
	subscriptions := graphql.Fields{}

	for group, crds := range crdsByGroup(crds) {

		queryGroupType := graphql.NewObject(graphql.ObjectConfig{
			Name:   group + "Type",
			Fields: graphql.Fields{},
		})

		mutationGroupType := graphql.NewObject(graphql.ObjectConfig{
			Name:   group + "Mutation",
			Fields: graphql.Fields{},
		})

		versionToQueryType := map[string]*graphql.Object{}
		versionToMutationType := map[string]*graphql.Object{}

		for _, crd := range crds {
			for _, typeInformation := range crd.Spec.Versions {
				if _, ok := versionToQueryType[typeInformation.Name]; ok {
					continue
				}

				versionToQueryType[typeInformation.Name] = graphql.NewObject(graphql.ObjectConfig{
					Name:   typeInformation.Name + "Type",
					Fields: graphql.Fields{},
				})

				versionToMutationType[typeInformation.Name] = graphql.NewObject(graphql.ObjectConfig{
					Name:   typeInformation.Name + "Mutation",
					Fields: graphql.Fields{},
				})
			}
		}

		for _, crd := range crds {

			for _, typeInformation := range crd.Spec.Versions {

				fields, inputFields := gqlTypeForOpenAPIProperties(typeInformation.Schema.OpenAPIV3Schema.Properties, graphql.Fields{}, graphql.InputObjectConfigFieldMap{}, cases.Title(language.English).String(crd.Spec.Names.Singular), nil)

				if len(fields) == 0 {
					slog.Info("skip processing of kind due to empty field map", "kind", crd.Spec.Names.Kind)
					continue
				}

				crdType := graphql.NewObject(graphql.ObjectConfig{
					Name:   crd.Spec.Names.Kind,
					Fields: fields,
				})

				crdType.AddFieldConfig("metadata", &graphql.Field{
					Type:        objectMeta,
					Description: "Standard object's metadata.",
				})

				versionedQueryType := versionToQueryType[typeInformation.Name]
				versionedMutationType := versionToMutationType[typeInformation.Name]

				versionedQueryType.AddFieldConfig(crd.Spec.Names.Plural, &graphql.Field{
					Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(crdType))),
					Args: graphql.FieldConfigArgument{
						"labelselector": &graphql.ArgumentConfig{
							Type:        graphql.String,
							Description: "a label selector to filter the objects by",
						},
						"namespace": &graphql.ArgumentConfig{
							Type:        graphql.String,
							Description: "the namespace in which to search for the objects",
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						ctx, span := otel.Tracer("").Start(p.Context, "Resolve", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
						defer span.End()

						listFunc, ok := conf.pluralToListType[crd.Spec.Names.Plural]
						if !ok {
							return nil, errors.New("no typed client available for the reuqested type")
						}

						list := listFunc()

						var opts []client.ListOption
						if labelSelector, ok := p.Args["labelselector"].(string); ok && labelSelector != "" {
							selector, err := labels.Parse(labelSelector)
							if err != nil {
								slog.Error("unable to parse given label selector", "error", err)
								return nil, err
							}
							opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
						}

						ras := authzv1.ResourceAttributes{
							Verb:     "list",
							Group:    crd.Spec.Group,
							Version:  typeInformation.Name,
							Resource: crd.Spec.Names.Plural,
						}
						if namespace, ok := p.Args["namespace"].(string); ok && namespace != "" {
							opts = append(opts, client.InNamespace(namespace))
							ras.Namespace = namespace
						}

						if err := isAuthorized(ctx, conf.Client, ras); err != nil {
							return nil, err
						}

						err = conf.Client.List(ctx, list, opts...)
						if err != nil {
							return nil, err
						}

						items, err := meta.ExtractList(list)
						if err != nil {
							return nil, err
						}

						// the controller-runtime cache returns unordered results so we sort it here
						slices.SortFunc(items, func(a runtime.Object, b runtime.Object) int {
							return strings.Compare(a.(client.Object).GetName(), b.(client.Object).GetName())
						})

						return items, nil
					},
				})

				versionedQueryType.AddFieldConfig(crd.Spec.Names.Singular, &graphql.Field{
					Type: graphql.NewNonNull(crdType),
					Args: graphql.FieldConfigArgument{
						"name": &graphql.ArgumentConfig{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "the metadata.name of the object you want to retrieve",
						},
						"namespace": &graphql.ArgumentConfig{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "the metadata.namesapce of the object you want to retrieve",
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
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

						if err := isAuthorized(ctx, conf.Client, authzv1.ResourceAttributes{
							Verb:      "get",
							Group:     crd.Spec.Group,
							Version:   typeInformation.Name,
							Resource:  crd.Spec.Names.Plural,
							Namespace: namespace,
							Name:      name,
						}); err != nil {
							return nil, err
						}

						objectFunc, ok := conf.pluralToObjectType[crd.Spec.Names.Plural]
						if !ok {
							return nil, errors.New("no typed client available for the reuqested type")
						}

						obj := objectFunc()
						err = conf.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj)
						if err != nil {
							return nil, err
						}

						return obj, nil
					},
				})

				capitalizedSingular := cases.Title(language.English).String(crd.Spec.Names.Singular)

				versionedMutationType.AddFieldConfig("delete"+capitalizedSingular, &graphql.Field{
					Type: graphql.Boolean,
					Args: graphql.FieldConfigArgument{
						"name": &graphql.ArgumentConfig{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "the metadata.name of the object you want to delete",
						},
						"namespace": &graphql.ArgumentConfig{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "the metadata.namesapce of the object you want to delete",
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						ctx, span := otel.Tracer("").Start(p.Context, "Delete", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
						defer span.End()

						if err := isAuthorized(ctx, conf.Client, authzv1.ResourceAttributes{
							Verb:      "delete",
							Group:     crd.Spec.Group,
							Version:   typeInformation.Name,
							Resource:  crd.Spec.Names.Plural,
							Namespace: p.Args["namespace"].(string),
							Name:      p.Args["name"].(string),
						}); err != nil {
							return nil, err
						}

						us := &unstructured.Unstructured{}
						us.SetGroupVersionKind(schema.GroupVersionKind{
							Group:   crd.Spec.Group,
							Version: typeInformation.Name,
							Kind:    crd.Spec.Names.Kind,
						})

						us.SetNamespace(p.Args["namespace"].(string))
						us.SetName(p.Args["name"].(string))

						err = conf.Client.Delete(ctx, us)

						return err == nil, err
					},
				})

				versionedMutationType.AddFieldConfig("create"+capitalizedSingular, &graphql.Field{
					Type: crdType,
					Args: graphql.FieldConfigArgument{
						"spec": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(inputFields["spec"].Type), // TODO: this is kind of unsafe, as we assume that the spec field is always present
						},
						"metadata": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(metadataInput),
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						ctx, span := otel.Tracer("").Start(p.Context, "Create", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
						defer span.End()

						var metadatInput MetadatInput
						if err := mapstructure.Decode(p.Args["metadata"], &metadatInput); err != nil {
							slog.Error("unable to decode metadata input", "error", err)
							return nil, err
						}

						if err := isAuthorized(ctx, conf.Client, authzv1.ResourceAttributes{
							Verb:      "create",
							Group:     crd.Spec.Group,
							Version:   typeInformation.Name,
							Resource:  crd.Spec.Names.Plural,
							Namespace: metadatInput.Namespace,
						}); err != nil {
							return nil, err
						}

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
							return nil, errors.New("either name or generateName must be set")
						}

						unstructured.SetNestedField(us.Object, p.Args["spec"], "spec")

						err = conf.Client.Create(ctx, us)

						return us.Object, err
					},
				})

				versionedMutationType.AddFieldConfig("update"+capitalizedSingular, &graphql.Field{
					Type: crdType,
					Args: graphql.FieldConfigArgument{
						"spec": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(inputFields["spec"].Type),
						},
						"metadata": &graphql.ArgumentConfig{
							Type: graphql.NewNonNull(metadataInput),
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						ctx, span := otel.Tracer("").Start(p.Context, "Update", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
						defer span.End()

						var metadatInput MetadatInput
						if err := mapstructure.Decode(p.Args["metadata"], &metadatInput); err != nil {
							slog.Error("unable to decode metadata input", "error", err)
							return nil, err
						}

						if err := isAuthorized(ctx, conf.Client, authzv1.ResourceAttributes{
							Verb:      "update",
							Group:     crd.Spec.Group,
							Version:   typeInformation.Name,
							Resource:  crd.Spec.Names.Plural,
							Namespace: metadatInput.Namespace,
							Name:      metadatInput.Name,
						}); err != nil {
							return nil, err
						}

						us := &unstructured.Unstructured{}
						us.SetGroupVersionKind(schema.GroupVersionKind{
							Group:   crd.Spec.Group,
							Version: typeInformation.Name,
							Kind:    crd.Spec.Names.Kind,
						})

						us.SetNamespace(metadatInput.Namespace)
						us.SetName(metadatInput.Name)

						err = conf.Client.Get(ctx, client.ObjectKey{Namespace: us.GetNamespace(), Name: us.GetName()}, us)
						if err != nil {
							return nil, err
						}

						unstructured.SetNestedField(us.Object, p.Args["spec"], "spec")

						err = conf.Client.Update(ctx, us)
						if err != nil {
							return nil, err
						}

						return us.Object, nil
					},
				})

				subscriptions[group+typeInformation.Name+capitalizedSingular] = &graphql.Field{
					Type: graphql.NewList(crdType),
					Args: graphql.FieldConfigArgument{
						"namespace": &graphql.ArgumentConfig{
							Type:        graphql.NewNonNull(graphql.String),
							Description: "the metadata.namesapce of the objects you want to watch",
						},
						"emitOnlyFieldChanges": &graphql.ArgumentConfig{
							Type:         graphql.Boolean,
							DefaultValue: false,
							Description:  "only emit events if the fields that are requested have changed",
						},
					},
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source, nil
					},
					Subscribe: func(p graphql.ResolveParams) (interface{}, error) {
						ctx, span := otel.Tracer("").Start(p.Context, "Subscribe", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
						defer span.End()

						listType, ok := conf.pluralToListType[crd.Spec.Names.Plural]
						if !ok {
							return nil, errors.New("no typed client available for the reuqested type")
						}

						if err := isAuthorized(ctx, conf.Client, authzv1.ResourceAttributes{
							Verb:      "watch",
							Group:     crd.Spec.Group,
							Version:   typeInformation.Name,
							Resource:  crd.Spec.Names.Plural,
							Namespace: p.Args["namespace"].(string),
						}); err != nil {
							return nil, err
						}

						listWatch, err := conf.Client.Watch(ctx, listType(), client.InNamespace(p.Args["namespace"].(string)))
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

												if val, ok := p.Args["emitOnlyFieldChanges"].(bool); ok && val {
													unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ev.Object)
													if err != nil {
														// TODO: handle error
														close(resultChannel)
													}

													fields := getRequestedFields(p)

													currentItemUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
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
														if fieldValue == currentFieldValue {
															continue
														}

														changed = true

													}
												}

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
					},
				}

				queryGroupType.AddFieldConfig(typeInformation.Name, &graphql.Field{
					Type:    graphql.NewNonNull(versionedQueryType),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) { return p.Source, nil },
				})

				mutationGroupType.AddFieldConfig(typeInformation.Name, &graphql.Field{
					Type:    graphql.NewNonNull(versionedMutationType),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) { return p.Source, nil },
				})
			}

		}

		rootQueryFields[group] = &graphql.Field{
			Type:    queryGroupType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) { return p.Source, nil },
		}
		rootMutationFields[group] = &graphql.Field{
			Type:    mutationGroupType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) { return p.Source, nil },
		}
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: rootQueryFields,
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: rootMutationFields,
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: subscriptions,
		}),
	})
}

type userContextKey struct{}

type HandlerConfig struct {
	*handler.Config
	UserClaim string
}

func Handler(conf HandlerConfig) http.Handler {
	h := handler.New(conf.Config)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" && (conf.Playground || conf.GraphiQL) {
			h.ServeHTTP(w, r)
			return
		}

		claims := jwt.MapClaims{}
		_, _, err := jwt.NewParser().ParseUnverified(token, claims)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		userIdentifier, ok := claims[conf.UserClaim].(string)
		if !ok || userIdentifier == "" {
			http.Error(w, "invalid user claim", http.StatusUnauthorized)
			return
		}

		ctx := AddUserToContext(r.Context(), userIdentifier)

		if r.Header.Get("Accept") == "text/event-stream" {
			opts := handler.NewRequestOptions(r)

			rc := http.NewResponseController(w)
			defer rc.Flush()

			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, ":\n\n")
			rc.Flush()

			subscriptionChannel := graphql.Subscribe(graphql.Params{
				Context:        ctx,
				Schema:         *conf.Schema,
				RequestString:  opts.Query,
				VariableValues: opts.Variables,
			})

			for result := range subscriptionChannel {
				b, _ := json.Marshal(result)
				fmt.Fprintf(w, "event: next\ndata: %s\n\n", b)
				rc.Flush()
			}

			fmt.Fprint(w, "event: complete\n\n")
			return
		}

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AddUserToContext(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func isAuthorized(ctx context.Context, c client.Client, resourceAttributes authzv1.ResourceAttributes) error {
	ctx, span := otel.Tracer("").Start(ctx, "AuthorizationCheck")
	defer span.End()

	user, ok := ctx.Value(userContextKey{}).(string)
	if !ok || user == "" {
		return errors.New("no user found in context")
	}

	sar := authzv1.SubjectAccessReview{
		Spec: authzv1.SubjectAccessReviewSpec{
			User:               user,
			ResourceAttributes: &resourceAttributes,
		},
	}

	err := c.Create(ctx, &sar)
	if err != nil {
		return err
	}
	slog.Info("SAR result", "allowed", sar.Status.Allowed, "user", sar.Spec.User, "namespace", sar.Spec.ResourceAttributes.Namespace, "resource", sar.Spec.ResourceAttributes.Resource)

	if !sar.Status.Allowed {
		return errors.New("access denied")
	}

	return nil
}
