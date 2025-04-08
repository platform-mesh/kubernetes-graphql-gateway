package cmd

import (
	"crypto/tls"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/discoveryclient"
	"k8s.io/client-go/discovery"
	"os"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/kcp"
)

func init() {
	rootCmd.AddCommand(listenCmd)
}

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	webhookServer        webhook.Server
	metricsServerOptions metricsserver.Options
	appCfg               config.Config
)

var listenCmd = &cobra.Command{
	Use:     "listener",
	Example: "KUBECONFIG=<path to kubeconfig file> go run . listener",
	PreRun: func(cmd *cobra.Command, args []string) {
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))

		utilruntime.Must(kcpapis.AddToScheme(scheme))
		utilruntime.Must(kcpcore.AddToScheme(scheme))
		utilruntime.Must(kcptenancy.AddToScheme(scheme))
		utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

		opts := zap.Options{
			Development: true,
		}
		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

		var err error
		appCfg, err = config.NewFromEnv()
		if err != nil {
			setupLog.Error(err, "failed to get operator flags from env, exiting...")
			os.Exit(1)
		}

		disableHTTP2 := func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		}

		var tlsOpts []func(*tls.Config)
		if !appCfg.EnableHTTP2 {
			tlsOpts = []func(c *tls.Config){disableHTTP2}
		}

		webhookServer = webhook.NewServer(webhook.Options{
			TLSOpts: tlsOpts,
		})

		metricsServerOptions = metricsserver.Options{
			BindAddress:   appCfg.MetricsAddr,
			SecureServing: appCfg.SecureMetrics,
			TLSOpts:       tlsOpts,
		}

		if appCfg.SecureMetrics {
			metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := ctrl.SetupSignalHandler()
		restCfg := ctrl.GetConfigOrDie()
		log, err := setupLogger(appCfg.LogLevel)
		if err != nil {
			setupLog.Error(err, "unable to setup logger")
			os.Exit(1)
		}

		mgrOpts := ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsServerOptions,
			WebhookServer:          webhookServer,
			HealthProbeBindAddress: appCfg.ProbeAddr,
			LeaderElection:         appCfg.EnableLeaderElection,
			LeaderElectionID:       "72231e1f.openmfp.io",
		}

		clt, err := client.New(restCfg, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			setupLog.Error(err, "failed to create client from config")
			os.Exit(1)
		}

		mf := kcp.NewManagerFactory(log, appCfg)

		mgr, err := mf.NewManager(ctx, restCfg, mgrOpts, clt)
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		discoveryInterface, err := discovery.NewDiscoveryClientForConfig(restCfg)
		if err != nil {
			setupLog.Error(err, "failed to create discovery client")
			os.Exit(1)
		}

		reconcilerOpts := kcp.ReconcilerOpts{
			Scheme:                 scheme,
			Client:                 clt,
			Config:                 restCfg,
			OpenAPIDefinitionsPath: appCfg.OpenApiDefinitionsPath,
		}

		reconciler, err := kcp.NewReconciler(
			ctx,
			log,
			appCfg,
			reconcilerOpts,
			discoveryInterface,
			kcp.PreReconcile,
			discoveryclient.NewFactory,
		)

		if err != nil {
			setupLog.Error(err, "unable to instantiate reconciler")
			os.Exit(1)
		}

		if err := reconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller")
			os.Exit(1)
		}

		if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up health check")
			os.Exit(1)
		}
		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up ready check")
			os.Exit(1)
		}

		setupLog.Info("starting manager")
		if err := mgr.Start(ctx); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	},
}
