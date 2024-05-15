package gateway

import (
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
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
	Description: "A map of strings",
	Serialize: func(value interface{}) interface{} {
		return value
	},
	ParseValue: func(value interface{}) interface{} {
		var out map[string]string
		switch value := value.(type) {
		case string:
			json.Unmarshal([]byte(value), &out)
		case *string:
			json.Unmarshal([]byte(*value), &out)
		default:
			return nil
		}
		return out
	},
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
			Type: graphql.NewNonNull(graphql.String),
		},
		"namespace": &graphql.Field{
			Type: graphql.NewNonNull(graphql.String),
		},
		"labels": &graphql.Field{
			Type: stringMapScalar,
		},
		"annotations": &graphql.Field{
			Type: stringMapScalar,
		},
	},
})

var metadataInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetadataInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"generateName": &graphql.InputObjectFieldConfig{
			Type: graphql.String,
		},
		"namespace": &graphql.InputObjectFieldConfig{
			Type: graphql.NewNonNull(graphql.String),
		},
		"labels": &graphql.InputObjectFieldConfig{
			Type: stringMapScalar,
		},
	},
})

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
			typeName := parentFieldName + cases.Title(language.English).String(key)
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
				Name:   parentFieldName + key + "Input",
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
	Client      client.WithWatch
	queryToType map[string]func() client.ObjectList
	UserClaim   string
}

func getListTypesFromScheme(schema runtime.Scheme, crds []apiextensionsv1.CustomResourceDefinition) map[string]func() client.ObjectList {
	pluralToList := map[string]func() client.ObjectList{}

	listInterface := reflect.TypeOf((*client.ObjectList)(nil)).Elem()

	for gvk, knownType := range schema.AllKnownTypes() {

		idx := slices.IndexFunc(crds, func(crd apiextensionsv1.CustomResourceDefinition) bool {
			return strings.Contains(gvk.Kind, crd.Spec.Names.Kind) && crd.Spec.Group == gvk.Group
		})
		if idx == -1 {
			continue
		}

		if !reflect.PointerTo(knownType).Implements(listInterface) {
			continue
		}

		pluralToList[crds[idx].Spec.Names.Plural] = func() client.ObjectList {
			return reflect.New(knownType).Interface().(client.ObjectList)
		}
	}

	return pluralToList
}

func FromCRDs(crds []apiextensionsv1.CustomResourceDefinition, conf Config) (graphql.Schema, error) {

	if conf.UserClaim == "" {
		conf.UserClaim = "mail"
	}

	conf.queryToType = getListTypesFromScheme(*conf.Client.Scheme(), crds)

	rootQueryFields := graphql.Fields{}
	rootMutationFields := graphql.Fields{}
	subscriptions := graphql.Fields{}

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

	for group, crds := range byGroup {

		queryGroupType := graphql.NewObject(graphql.ObjectConfig{
			Name: group + "Type",
			Fields: graphql.Fields{
				"debug": &graphql.Field{
					Type: graphql.String,
				},
			},
		})

		mutationGroupType := graphql.NewObject(graphql.ObjectConfig{
			Name: group + "Mutation",
			Fields: graphql.Fields{
				"debug": &graphql.Field{
					Type: graphql.String,
				},
			},
		})

		for _, crd := range crds {

			versionIdx := slices.IndexFunc(crd.Spec.Versions, func(version apiextensionsv1.CustomResourceDefinitionVersion) bool { return version.Storage })
			typeInformation := crd.Spec.Versions[versionIdx]

			fields, inputFields := gqlTypeForOpenAPIProperties(typeInformation.Schema.OpenAPIV3Schema.Properties, graphql.Fields{}, graphql.InputObjectConfigFieldMap{}, crd.Spec.Names.Kind, nil)

			if len(fields) == 0 {
				slog.Info("skip processing of kind due to empty field map", "kind", crd.Spec.Names.Kind)
				continue
			}

			crdType := graphql.NewObject(graphql.ObjectConfig{
				Name:   crd.Spec.Names.Kind,
				Fields: fields,
			})

			crdType.AddFieldConfig("metadata", &graphql.Field{
				Type: objectMeta,
			})

			queryGroupType.AddFieldConfig(crd.Spec.Names.Plural, &graphql.Field{
				Type: graphql.NewList(crdType),
				Args: graphql.FieldConfigArgument{
					"labelselector": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"namespace": &graphql.ArgumentConfig{
						Type:        graphql.String,
						Description: "the namespace in which to search for the objects",
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx, span := otel.Tracer("").Start(p.Context, "Resolve", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
					defer span.End()

					listFunc, ok := conf.queryToType[crd.Spec.Names.Plural]
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

					claims := jwt.MapClaims{}
					_, _, err := jwt.NewParser().ParseUnverified(p.Info.RootValue.(map[string]interface{})["token"].(string), &claims)
					if err != nil {
						return nil, err
					}

					sar := authzv1.SubjectAccessReview{
						Spec: authzv1.SubjectAccessReviewSpec{
							// TODO: make this conversion more robust
							User: claims[conf.UserClaim].(string),
							ResourceAttributes: &authzv1.ResourceAttributes{
								Verb:     "list",
								Group:    crd.Spec.Group,
								Version:  crd.Spec.Versions[versionIdx].Name,
								Resource: crd.Spec.Names.Plural,
							},
						},
					}

					if namespace, ok := p.Args["namespace"].(string); ok && namespace != "" {
						opts = append(opts, client.InNamespace(namespace))
						sar.Spec.ResourceAttributes.Namespace = namespace
					}

					err = conf.Client.Create(ctx, &sar)
					if err != nil {
						return nil, err
					}
					slog.Info("SAR result", "allowed", sar.Status.Allowed, "user", sar.Spec.User, "namespace", sar.Spec.ResourceAttributes.Namespace, "resource", sar.Spec.ResourceAttributes.Resource)

					if !sar.Status.Allowed {
						return nil, errors.New("access denied")
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

			subscriptions[group+crd.Spec.Names.Kind] = &graphql.Field{
				Type: crdType,
				Args: graphql.FieldConfigArgument{
					"name": &graphql.ArgumentConfig{
						Type:        graphql.NewNonNull(graphql.String),
						Description: "the metadata.name of the object you want to watch",
					},
					"namespace": &graphql.ArgumentConfig{
						Type:        graphql.NewNonNull(graphql.String),
						Description: "the metadata.namesapce of the object you want to watch",
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return p.Source, nil
				},
				Subscribe: func(p graphql.ResolveParams) (interface{}, error) {
					ctx, span := otel.Tracer("").Start(p.Context, "Subscribe", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
					defer span.End()

					listType, ok := conf.queryToType[crd.Spec.Names.Plural]
					if !ok {
						return nil, errors.New("no typed client available for the reuqested type")
					}

					list := listType()

					claims := jwt.MapClaims{}
					_, _, err := jwt.NewParser().ParseUnverified(p.Info.RootValue.(map[string]interface{})["token"].(string), &claims)
					if err != nil {
						return nil, err
					}

					sar := authzv1.SubjectAccessReview{
						Spec: authzv1.SubjectAccessReviewSpec{
							// TODO: make this conversion more robust
							User: claims[conf.UserClaim].(string),
							ResourceAttributes: &authzv1.ResourceAttributes{
								Verb:      "watch",
								Group:     crd.Spec.Group,
								Version:   crd.Spec.Versions[versionIdx].Name,
								Resource:  crd.Spec.Names.Plural,
								Namespace: p.Args["namespace"].(string),
								Name:      p.Args["name"].(string),
							},
						},
					}

					err = conf.Client.Create(ctx, &sar)
					if err != nil {
						return nil, err
					}
					slog.Info("SAR result", "allowed", sar.Status.Allowed, "user", sar.Spec.User, "namespace", sar.Spec.ResourceAttributes.Namespace, "resource", sar.Spec.ResourceAttributes.Resource)

					if !sar.Status.Allowed {
						return nil, errors.New("access denied")
					}

					listWatch, err := conf.Client.Watch(ctx, list, client.InNamespace(p.Args["namespace"].(string)), client.MatchingFields{
						"metadata.name": p.Args["name"].(string),
					})
					if err != nil {
						return nil, err
					}

					resultChannel := make(chan interface{})
					go func() {
						for ev := range listWatch.ResultChan() {
							select {
							case <-ctx.Done():
								slog.Info("stopping watch due to client cancel")
								listWatch.Stop()
								close(resultChannel)
							default:
								if ev.Type == watch.Bookmark {
									continue
								}

								resultChannel <- ev.Object
							}
						}
					}()

					return resultChannel, nil
				},
			}

			mutationGroupType.AddFieldConfig("delete"+crd.Spec.Names.Kind, &graphql.Field{
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

					claims := jwt.MapClaims{}
					_, _, err := jwt.NewParser().ParseUnverified(p.Info.RootValue.(map[string]interface{})["token"].(string), &claims)
					if err != nil {
						return nil, err
					}

					sar := authzv1.SubjectAccessReview{
						Spec: authzv1.SubjectAccessReviewSpec{
							// TODO: make this conversion more robust
							User: claims[conf.UserClaim].(string),
							ResourceAttributes: &authzv1.ResourceAttributes{
								Verb:      "delete",
								Group:     crd.Spec.Group,
								Version:   crd.Spec.Versions[versionIdx].Name,
								Resource:  crd.Spec.Names.Plural,
								Namespace: p.Args["namespace"].(string),
								Name:      p.Args["name"].(string),
							},
						},
					}

					err = conf.Client.Create(ctx, &sar)
					if err != nil {
						return nil, err
					}
					slog.Info("SAR result", "allowed", sar.Status.Allowed, "user", sar.Spec.User, "namespace", sar.Spec.ResourceAttributes.Namespace, "resource", sar.Spec.ResourceAttributes.Resource)

					if !sar.Status.Allowed {
						return nil, errors.New("access denied")
					}

					us := &unstructured.Unstructured{}
					us.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   crd.Spec.Group,
						Version: crd.Spec.Versions[versionIdx].Name,
						Kind:    crd.Spec.Names.Kind,
					})

					us.SetNamespace(p.Args["namespace"].(string))
					us.SetName(p.Args["name"].(string))

					err = conf.Client.Delete(ctx, us)

					return err == nil, err
				},
			})

			mutationGroupType.AddFieldConfig("create"+crd.Spec.Names.Kind, &graphql.Field{
				Type: crdType,
				Args: graphql.FieldConfigArgument{
					"spec": &graphql.ArgumentConfig{
						Type: inputFields["spec"].Type,
					},
					"metadata": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(metadataInput),
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx, span := otel.Tracer("").Start(p.Context, "Create", trace.WithAttributes(attribute.String("kind", crd.Spec.Names.Kind)))
					defer span.End()

					claims := jwt.MapClaims{}
					_, _, err := jwt.NewParser().ParseUnverified(p.Info.RootValue.(map[string]interface{})["token"].(string), &claims)
					if err != nil {
						return nil, err
					}

					sar := authzv1.SubjectAccessReview{
						Spec: authzv1.SubjectAccessReviewSpec{
							// TODO: make this conversion more robust
							User: claims[conf.UserClaim].(string),
							ResourceAttributes: &authzv1.ResourceAttributes{
								Verb:      "create",
								Group:     crd.Spec.Group,
								Version:   crd.Spec.Versions[versionIdx].Name,
								Resource:  crd.Spec.Names.Plural,
								Namespace: p.Args["metadata"].(map[string]interface{})["namespace"].(string),
							},
						},
					}

					err = conf.Client.Create(ctx, &sar)
					if err != nil {
						return nil, err
					}
					slog.Info("SAR result", "allowed", sar.Status.Allowed, "user", sar.Spec.User, "namespace", sar.Spec.ResourceAttributes.Namespace, "resource", sar.Spec.ResourceAttributes.Resource)

					if !sar.Status.Allowed {
						return nil, errors.New("access denied")
					}

					us := &unstructured.Unstructured{}
					us.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   crd.Spec.Group,
						Version: crd.Spec.Versions[versionIdx].Name,
						Kind:    crd.Spec.Names.Kind,
					})

					us.SetNamespace(p.Args["metadata"].(map[string]interface{})["namespace"].(string))
					if name := p.Args["metadata"].(map[string]interface{})["name"]; name != nil {
						us.SetName(name.(string))
					}

					if generateName := p.Args["metadata"].(map[string]interface{})["generateName"]; generateName != nil {
						us.SetGenerateName(generateName.(string))
					}

					if labels := p.Args["metadata"].(map[string]interface{})["labels"]; labels != nil {
						us.SetLabels(labels.(map[string]string))
					}

					if us.GetName() == "" && us.GetGenerateName() == "" {
						return nil, errors.New("either name or generateName must be set")
					}

					unstructured.SetNestedField(us.Object, p.Args["spec"], "spec")

					err = conf.Client.Create(ctx, us)

					return us.Object, err
				},
			})
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
