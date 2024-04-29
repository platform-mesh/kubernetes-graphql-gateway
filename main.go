package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/graphql-go/handler"
	"github.com/openmfp/crd-gql-gateway/gateway"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {

	cfg := controllerruntime.GetConfigOrDie()

	schema := runtime.NewScheme()
	apiextensionsv1.AddToScheme(schema)

	cl, err := client.New(cfg, client.Options{
		Scheme: schema,
		Cache: &client.CacheOptions{
			Unstructured: true,
		},
	})
	if err != nil {
		panic(err)
	}

	var crdList apiextensionsv1.CustomResourceDefinitionList
	err = cl.List(context.Background(), &crdList)
	if err != nil {
		panic(err)
	}

	var crds []apiextensionsv1.CustomResourceDefinition
	for _, crd := range crdList.Items {
		if strings.Contains(crd.Spec.Group, "sap.com") {
			crds = append(crds, crd)
		}
	}

	gqlSchema, err := gateway.FromCRDs(crds, cl)
	if err != nil {
		panic(err)
	}

	http.Handle("/graphql", handler.New(&handler.Config{
		Schema:     &gqlSchema,
		Pretty:     true,
		GraphiQL:   false,
		Playground: true,
	}))

	http.ListenAndServe(":3000", nil)
}
