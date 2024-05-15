package cmd

import (
	"net/http"

	"context"
	"strings"

	"github.com/spf13/cobra"

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

var startCmd = &cobra.Command{
	Use: "start",
	RunE: func(cmd *cobra.Command, args []string) error {

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
			return err
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
			return err
		}

		gqlSchema, err := gateway.New(cmd.Context(), gateway.Config{
			Client: cl,
		})
		if err != nil {
			return err
		}

		http.Handle("/graphql", handler.New(&handler.Config{
			Schema:     &gqlSchema,
			Pretty:     true,
			Playground: true,
			RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
				// TODO: it would be great to pass that via the context instead of the RootObjectFn
				return map[string]interface{}{
					"token": strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "),
				}
			},
		}))

		http.Handle("/subscription", transport.New(gqlSchema))

		return http.ListenAndServe(":3000", nil)
	},
}
