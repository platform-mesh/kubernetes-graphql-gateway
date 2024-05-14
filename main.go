package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/openmfp/crd-gql-gateway/gateway"
	"github.com/openmfp/crd-gql-gateway/transport"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jirav1alpha1 "github.tools.sap/automaticd/automaticd/operators/jira/api/v1alpha1"
	echelonv1alpha "github.tools.sap/dxp/echelon-operator/api/v1alpha1"
	extensionsv1alpha1 "github.tools.sap/dxp/extension-manager-operator/api/extensions.dxp.sap.com/v1alpha1"
	authzv1 "k8s.io/api/authorization/v1"
)

func main() {

	cfg := controllerruntime.GetConfigOrDie()

	schema := runtime.NewScheme()
	apiextensionsv1.AddToScheme(schema)
	echelonv1alpha.AddToScheme(schema)
	extensionsv1alpha1.AddToScheme(schema)
	jirav1alpha1.AddToScheme(schema)
	authzv1.AddToScheme(schema)

	k8sCache, err := cache.New(cfg, cache.Options{
		Scheme: schema,
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
		if strings.Contains(crd.Spec.Group, "automaticd.sap") {
			crds = append(crds, crd)
		}
	}

	mapping := map[string]func() client.ObjectList{
		"jiraprojects":                  func() client.ObjectList { return &jirav1alpha1.JiraProjectList{} },
		"accounts":                      func() client.ObjectList { return &echelonv1alpha.AccountList{} },
		"extensionclasses":              func() client.ObjectList { return &extensionsv1alpha1.ExtensionClassList{} },
		"fabricFoundationSapComAccount": func() client.ObjectList { return &echelonv1alpha.AccountList{} }, // REFACTOR: this is a hack for the subscriptions which I do not like
		"automaticdSapJiraProject":      func() client.ObjectList { return &jirav1alpha1.JiraProjectList{} },
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
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			return map[string]interface{}{
				"token": strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "),
			}
		},
	}))

	http.Handle("/subscription", transport.New(gqlSchema))

	http.ListenAndServe(":3000", nil)
}
