package gateway

import (
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/graphql-go/graphql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	authzv1 "k8s.io/api/authorization/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var objectMeta = graphql.NewObject(graphql.ObjectConfig{
	Name: "Metadata",
	Fields: graphql.Fields{
		"name": &graphql.Field{
			Type: graphql.NewNonNull(graphql.String),
		},
		"namespace": &graphql.Field{
			Type: graphql.NewNonNull(graphql.String),
		},
	},
})

func gqlTypeForOpenAPIProperties(in map[string]apiextensionsv1.JSONSchemaProps, fields graphql.Fields, parentFieldName string, requiredKeys []string) graphql.Fields {
	for key, info := range in {
		typeKey := strings.ReplaceAll(key, "-", "")
		currentField := &graphql.Field{
			Name:        typeKey,
			Description: info.Description,
		}

		switch info.Type {
		case "string":
			currentField.Type = graphql.String
		case "boolean":
			currentField.Type = graphql.Boolean
		case "integer":
			currentField.Type = graphql.Int
		case "array":
			typeName := parentFieldName + cases.Title(language.English).String(key) + "Item"

			if info.Items.Schema.Properties != nil { // nested array object
				newFields := gqlTypeForOpenAPIProperties(info.Items.Schema.Properties, graphql.Fields{}, typeName, info.Items.Schema.Required)
				newType := graphql.NewObject(graphql.ObjectConfig{
					Name:   typeName,
					Fields: newFields,
				})
				if len(newFields) == 0 {
					slog.Info("skipping creation of subtype due to emtpy field configuration", "type", typeName)
					continue
				}

				currentField.Type = graphql.NewList(newType)
			} else { // primitive array
				switch info.Items.Schema.Type {
				case "string":
					currentField.Type = graphql.String
				case "boolean":
					currentField.Type = graphql.Boolean
				case "integer":
					currentField.Type = graphql.Int
				}

				currentField.Type = graphql.NewList(currentField.Type)
			}
		case "object":
			if info.Properties == nil {
				continue
			}
			typeName := parentFieldName + cases.Title(language.English).String(key)
			newFields := gqlTypeForOpenAPIProperties(info.Properties, graphql.Fields{}, typeName, info.Required)
			if len(newFields) == 0 {
				slog.Info("skipping creation of subtype due to emtpy field configuration", "type", typeName)
				continue
			}
			newType := graphql.NewObject(graphql.ObjectConfig{
				Name:   parentFieldName + key,
				Fields: newFields,
			})
			currentField.Type = newType
		default:
			continue
		}

		if slices.Contains(requiredKeys, key) {
			currentField.Type = graphql.NewNonNull(currentField.Type)
		}

		fields[typeKey] = currentField
	}

	return fields
}

type Config struct {
	Client      client.WithWatch
	QueryToType map[string]func() client.ObjectList
	UserClaim   string
}

func FromCRDs(crds []apiextensionsv1.CustomResourceDefinition, conf Config) (graphql.Schema, error) {

	if conf.UserClaim == "" {
		conf.UserClaim = "mail"
	}

	rootQueryFields := graphql.Fields{}
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

		for _, crd := range crds {

			versionIdx := slices.IndexFunc(crd.Spec.Versions, func(version apiextensionsv1.CustomResourceDefinitionVersion) bool { return version.Storage })
			typeInformation := crd.Spec.Versions[versionIdx]

			fields := gqlTypeForOpenAPIProperties(typeInformation.Schema.OpenAPIV3Schema.Properties, graphql.Fields{}, crd.Spec.Names.Kind, nil)

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

					listFunc, ok := conf.QueryToType[crd.Spec.Names.Plural]
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

					listType, ok := conf.QueryToType[crd.Spec.Names.Plural]
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
		}

		rootQueryFields[group] = &graphql.Field{
			Type:    queryGroupType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) { return p.Source, nil },
		}
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: rootQueryFields,
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: subscriptions,
		}),
	})
}
