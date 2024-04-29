package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"

	"github.com/graphql-go/graphql"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
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
	Client          client.Client
	QueryToTypeFunc func(graphql.ResolveParams) (client.ObjectList, error)
}

func FromCRDs(crds []apiextensionsv1.CustomResourceDefinition, conf Config) (graphql.Schema, error) {
	query := graphql.NewObject(graphql.ObjectConfig{
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

		query.AddFieldConfig(crd.Spec.Names.Plural, &graphql.Field{
			Type: graphql.NewList(crdType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var us unstructured.UnstructuredList
				idx := slices.IndexFunc(crds, func(crd apiextensionsv1.CustomResourceDefinition) bool {
					return crd.Spec.Names.Plural == p.Info.FieldName
				})

				us.SetGroupVersionKind(runtimeschema.GroupVersionKind{
					Group:   crds[idx].Spec.Group,
					Version: crds[idx].Spec.Versions[0].Name,
					Kind:    crds[idx].Spec.Names.Kind,
				})

				list, err := conf.QueryToTypeFunc(p)
				if err != nil {
					return nil, err
				}

				err = conf.Client.List(context.Background(), list)
				if err != nil {
					return nil, err
				}

				// TODO: subject access review

				// FIXME: this is currently a workaround to see if the typedClients are working
				// this method loses the order of the elements which is unintended
				result := map[string]any{}
				var intermediate bytes.Buffer
				json.NewEncoder(&intermediate).Encode(list)
				json.NewDecoder(&intermediate).Decode(&result)

				return result["items"], nil
			},
		})
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: query,
	})
}
