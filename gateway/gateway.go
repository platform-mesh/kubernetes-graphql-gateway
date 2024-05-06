package gateway

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/graphql-go/graphql"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	Client          client.WithWatch
	QueryToTypeFunc func(graphql.ResolveParams) (client.ObjectList, error)
}

func FromCRDs(crds []apiextensionsv1.CustomResourceDefinition, conf Config) (graphql.Schema, error) {

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
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					list, err := conf.QueryToTypeFunc(p)
					if err != nil {
						return nil, err
					}

					var opts []client.ListOption
					if labelSelector, ok := p.Args["labelselector"].(string); ok && labelSelector != "" {
						selector, err := labels.Parse(labelSelector)
						if err != nil {
							slog.Error("unable to parse given label selector", "error", err)
							return nil, err
						}
						opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
					}

					err = conf.Client.List(p.Context, list, opts...)
					if err != nil {
						return nil, err
					}

					// TODO: subject access review

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
					list, err := conf.QueryToTypeFunc(p)
					if err != nil {
						return nil, err
					}

					// TODO: SAR before staring the watch

					listWatch, err := conf.Client.Watch(p.Context, list, client.InNamespace(p.Args["namespace"].(string)), client.MatchingFields{
						"metadata.name": p.Args["name"].(string),
					})
					if err != nil {
						return nil, err
					}

					resultChannel := make(chan interface{})
					go func() {
						for ev := range listWatch.ResultChan() {
							select {
							case <-p.Context.Done():
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
