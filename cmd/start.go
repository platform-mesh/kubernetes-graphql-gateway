package cmd

import (
	"net/http"

	"context"

	"github.com/graphql-go/handler"
	"github.com/spf13/cobra"

	"github.com/openmfp/crd-gql-gateway/deprecated"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authzv1 "k8s.io/api/authorization/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var startCmd = &cobra.Command{
	Use: "start",
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg := controllerruntime.GetConfigOrDie()

		schema := runtime.NewScheme()
		utilruntime.Must(apiextensionsv1.AddToScheme(schema))
		utilruntime.Must(authzv1.AddToScheme(schema))

		k8sCache, err := cache.New(cfg, cache.Options{
			Scheme: schema,
		})
		if err != nil {
			return err
		}

		go func() {
			err := k8sCache.Start(context.Background())
			if err != nil {
				panic(err)
			}
		}()

		if !k8sCache.WaitForCacheSync(context.Background()) {
			panic("no cache sync")
		}

		cfg.Wrap(deprecated.NewImpersonationTransport)

		cl, err := client.NewWithWatch(cfg, client.Options{
			Scheme: schema,
			Cache: &client.CacheOptions{
				Reader: k8sCache,
			},
		})
		if err != nil {
			return err
		}

		gqlSchema, err := deprecated.New(cmd.Context(), deprecated.Config{
			Client: cl,
		})
		if err != nil {
			return err
		}

		http.Handle("/graphql", deprecated.Handler(deprecated.HandlerConfig{
			Config: &handler.Config{
				Schema:     &gqlSchema,
				Pretty:     true,
				Playground: true,
			},
			UserClaim:   "mail",
			GroupsClaim: "groups",
		}))

		return http.ListenAndServe(":3000", nil)
	},
}
