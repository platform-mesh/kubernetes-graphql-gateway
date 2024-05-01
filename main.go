package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/openmfp/crd-gql-gateway/gateway"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	echelonv1alpha "github.tools.sap/dxp/echelon-operator/api/v1alpha1"
	extensionsv1alpha1 "github.tools.sap/dxp/extension-manager-operator/api/extensions.dxp.sap.com/v1alpha1"
)

func main() {

	cfg := controllerruntime.GetConfigOrDie()

	schema := runtime.NewScheme()
	apiextensionsv1.AddToScheme(schema)
	echelonv1alpha.AddToScheme(schema)
	extensionsv1alpha1.AddToScheme(schema)

	k8sCache, err := cache.New(cfg, cache.Options{
		Scheme: schema,
		ByObject: map[client.Object]cache.ByObject{
			&echelonv1alpha.Account{}:            {},
			&extensionsv1alpha1.ExtensionClass{}: {},
		},
	})
	if err != nil {
		panic(err)
	}

	go func() {
		k8sCache.Start(context.Background())
	}()

	if !k8sCache.WaitForCacheSync(context.Background()) {
		panic("no cache sync")
	}

	cl, err := client.NewWithWatch(cfg, client.Options{
		Scheme: schema,
		Cache: &client.CacheOptions{
			Reader: k8sCache,
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

	mapping := map[string]func() client.ObjectList{
		"accounts":                      func() client.ObjectList { return &echelonv1alpha.AccountList{} },
		"extensionclasses":              func() client.ObjectList { return &extensionsv1alpha1.ExtensionClassList{} },
		"fabricFoundationSapComAccount": func() client.ObjectList { return &echelonv1alpha.AccountList{} }, // REFACTOR: this is a hack for the subscriptions which I do not like
	}

	gqlSchema, err := gateway.FromCRDs(crds, gateway.Config{
		Client: cl,
		QueryToTypeFunc: func(rp graphql.ResolveParams) (client.ObjectList, error) {
			f := mapping[rp.Info.FieldName]
			if f == nil {
				return nil, errors.New("no typed client available for the reuqested type")
			}
			return f(), nil
		},
	})
	if err != nil {
		panic(err)
	}

	http.Handle("/graphql", handler.New(&handler.Config{
		Schema:     &gqlSchema,
		Pretty:     true,
		Playground: true,
	}))
	http.HandleFunc("/subscription", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		opts := handler.NewRequestOptions(r)

		rc := http.NewResponseController(w)
		defer rc.Flush()

		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "application/json")

		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ":\n\n")
		rc.Flush()

		subscriptionChannel := graphql.Subscribe(graphql.Params{
			Schema:         gqlSchema,
			Context:        ctx,
			RequestString:  opts.Query,
			VariableValues: opts.Variables,
		})

		for result := range subscriptionChannel {
			b, _ := json.Marshal(result)
			fmt.Fprintf(w, "event: next\ndata: %s\n\n", b)
			rc.Flush()
		}

		fmt.Fprint(w, "event: complete\n\n")
	})

	http.ListenAndServe(":3000", nil)
}
