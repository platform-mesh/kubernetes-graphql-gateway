package gateway

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/graphql-go/graphql"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func gqlTypeForOpenAPIProperties(in map[string]apiextensionsv1.JSONSchemaProps, fields graphql.Fields, parentFieldName string) graphql.Fields {
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
		case "object":
			if in[key].Properties == nil {
				continue
			}
			typeName := parentFieldName + cases.Title(language.English).String(key)
			newFields := gqlTypeForOpenAPIProperties(in[key].Properties, graphql.Fields{}, typeName)
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

		fields[typeKey] = currentField
	}

	return fields
}

type Config struct {
	Client          client.WithWatch
	QueryToTypeFunc func(graphql.ResolveParams) (client.ObjectList, error)
}

func FromCRDs(crds []apiextensionsv1.CustomResourceDefinition, conf Config) (graphql.Schema, error) {
	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"version": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "dev", nil
				},
			},
		},
	})

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

		groupType := graphql.NewObject(graphql.ObjectConfig{
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

			fields := gqlTypeForOpenAPIProperties(typeInformation.Schema.OpenAPIV3Schema.Properties, graphql.Fields{}, crd.Spec.Names.Kind)

			if len(fields) == 0 {
				slog.Info("skip processing of kind due to empty field map", "kind", crd.Spec.Names.Kind)
				continue
			}

			crdType := graphql.NewObject(graphql.ObjectConfig{
				Name:   crd.Spec.Names.Kind,
				Fields: fields,
			})

			groupType.AddFieldConfig(crd.Spec.Names.Plural, &graphql.Field{
				Type: graphql.NewList(crdType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					list, err := conf.QueryToTypeFunc(p)
					if err != nil {
						return nil, err
					}

					err = conf.Client.List(p.Context, list)
					if err != nil {
						return nil, err
					}

					// TODO: subject access review

					// FIXME: this looses ordering of the results
					result, err := runtime.DefaultUnstructuredConverter.ToUnstructured(list)
					if err != nil {
						return nil, err
					}

					return result["items"], nil
				},
			})

			subscriptions[group+crd.Spec.Names.Kind] = &graphql.Field{
				Type: crdType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return p.Source, nil
				},
				Subscribe: func(p graphql.ResolveParams) (interface{}, error) {
					list, err := conf.QueryToTypeFunc(p)
					if err != nil {
						return nil, err
					}

					listWatch, err := conf.Client.Watch(p.Context, list)
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

								r, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ev.Object)
								if err != nil {
									listWatch.Stop()
									slog.Error("error converting", "err", err.Error())
									close(resultChannel)
									return
								}
								resultChannel <- r
							}
						}
					}()

					return resultChannel, nil
				},
			}
		}

		rootQuery.AddFieldConfig(group, &graphql.Field{
			Type:    groupType,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) { return p.Source, nil },
		})
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: rootQuery,
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: subscriptions,
		}),
	})
}
