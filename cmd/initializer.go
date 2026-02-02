package cmd

import (
	"crypto/tls"

	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler/kcp"
	"github.com/spf13/cobra"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	kcpapis "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/sdk/apis/tenancy/v1alpha1"

	"github.com/kcp-dev/multicluster-provider/initializingworkspaces"
)

var initializerCmd = &cobra.Command{
	Use:   "initializer",
	Short: "KCP Initializer for workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Str("LogLevel", log.GetLevel().String()).Msg("Starting the Initializer...")

		ctx := ctrl.SetupSignalHandler()
		restCfg := ctrl.GetConfigOrDie()

		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))

		if appCfg.EnableKcp {
			utilruntime.Must(kcpapis.AddToScheme(scheme))
			utilruntime.Must(kcpcore.AddToScheme(scheme))
			utilruntime.Must(kcptenancy.AddToScheme(scheme))
		}

		utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
		utilruntime.Must(gatewayv1alpha1.AddToScheme(scheme))

		ctrl.SetLogger(log.ComponentLogger("controller-runtime").Logr())

		disableHTTP2 := func(c *tls.Config) {
			log.Info().Msg("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		}

		var tlsOpts []func(*tls.Config)
		if !defaultCfg.EnableHTTP2 {
			tlsOpts = []func(c *tls.Config){disableHTTP2}
		}

		metricsServerOptions := metricsserver.Options{
			BindAddress:   defaultCfg.Metrics.BindAddress,
			SecureServing: defaultCfg.Metrics.Secure,
			TLSOpts:       tlsOpts,
		}

		if defaultCfg.Metrics.Secure {
			metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
		}

		mgrOpts := ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsServerOptions,
			HealthProbeBindAddress: defaultCfg.HealthProbeBindAddress,
			LeaderElection:         defaultCfg.LeaderElection.Enabled,
			LeaderElectionID:       "initializer.platform-mesh.io",
		}

		provider, err := initializingworkspaces.New(restCfg, "root:orgs",
			initializingworkspaces.Options{
				Scheme: mgrOpts.Scheme,
			},
		)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to construct cluster provider")
		}

		mgr, err := mcmanager.New(restCfg, provider, mgrOpts)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create multi-cluster manager")
		}

		ioHandler, err := workspacefile.NewIOHandler(appCfg.OpenApiDefinitionsPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create IO handler")
		}

		schemaResolver := apischema.NewResolver(log)

		clusterPathResolver, err := kcp.NewClusterPathResolver(restCfg, scheme)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create cluster path resolver")
		}

		discoveryFactory, err := kcp.NewDiscoveryFactory(restCfg)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create discovery factory")
		}

		initializingWSReconciler := &kcp.InitializingWorkspacesReconciler{
			Client:              nil,
			DiscoveryFactory:    discoveryFactory,
			APISchemaResolver:   schemaResolver,
			ClusterPathResolver: clusterPathResolver,
			IOHandler:           ioHandler,
			Log:                 log,
		}

		if appCfg.Listener.InitializingWorkspacesQueueURL != "" {
			initializingWSMgr, err := kcp.NewInitializingWorkspacesManager(appCfg.Listener.InitializingWorkspacesQueueURL, restCfg, scheme)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create initializing workspaces manager")
			}
			initializingWSReconciler.Client = initializingWSMgr.GetClient()

			if err := ctrl.NewControllerManagedBy(initializingWSMgr).
				For(&kcpcore.LogicalCluster{}).
				Complete(initializingWSReconciler); err != nil {
				log.Fatal().Err(err).Msg("failed to setup InitializingWorkspaces controller")
			}

			go func() {
				log.Info().Msg("starting initializing workspaces manager")
				if err := initializingWSMgr.Start(ctx); err != nil {
					log.Fatal().Err(err).Msg("problem running initializing workspaces manager")
				}
			}()
		} else {
			initializingWSReconciler.Client = mgr.GetLocalManager().GetClient()

			if err := ctrl.NewControllerManagedBy(mgr.GetLocalManager()).
				For(&kcpcore.LogicalCluster{}).
				Complete(initializingWSReconciler); err != nil {
				log.Fatal().Err(err).Msg("failed to setup InitializingWorkspaces controller")
			}
		}

		if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			log.Fatal().Err(err).Msg("unable to set up health check")
		}

		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			log.Fatal().Err(err).Msg("unable to set up ready check")
		}

		log.Info().Msg("starting manager")
		if err := mgr.Start(ctx); err != nil {
			log.Fatal().Err(err).Msg("problem running manager")
		}
	},
}
