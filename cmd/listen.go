package cmd

import (
	"crypto/tls"
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
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/openmfp/crd-gql-gateway/listener/flags"
	"github.com/openmfp/crd-gql-gateway/listener/kcp"
	// +kubebuilder:scaffold:imports
)

func init() {
	rootCmd.AddCommand(listenCmd)
}

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	webhookServer        webhook.Server
	metricsServerOptions metricsserver.Options
	opFlags              *flags.Flags
)

var listenCmd = &cobra.Command{
	Use:     "listen",
	Example: "KUBECONFIG=<path to kubeconfig file> go run . listen",
	PreRun: func(cmd *cobra.Command, args []string) {
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))

		utilruntime.Must(kcpapis.AddToScheme(scheme))
		utilruntime.Must(kcpcore.AddToScheme(scheme))
		utilruntime.Must(kcptenancy.AddToScheme(scheme))
		utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
		// +kubebuilder:scaffold:scheme

		opts := zap.Options{
			Development: true,
		}
		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

		var err error
		opFlags, err = flags.NewFromEnv()
		if err != nil {
			setupLog.Error(err, "failed to get operator flags from env, exiting...")
			os.Exit(1)
		}

		disableHTTP2 := func(c *tls.Config) {
			setupLog.Info("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		}

		var tlsOpts []func(*tls.Config)
		if !opFlags.EnableHTTP2 {
			tlsOpts = []func(c *tls.Config){disableHTTP2}
		}

		webhookServer = webhook.NewServer(webhook.Options{
			TLSOpts: tlsOpts,
		})

		metricsServerOptions = metricsserver.Options{
			BindAddress:   opFlags.MetricsAddr,
			SecureServing: opFlags.SecureMetrics,
			TLSOpts:       tlsOpts,
		}

		if opFlags.SecureMetrics {
			metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := ctrl.GetConfigOrDie()

		mgrOpts := ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsServerOptions,
			WebhookServer:          webhookServer,
			HealthProbeBindAddress: opFlags.ProbeAddr,
			LeaderElection:         opFlags.EnableLeaderElection,
			LeaderElectionID:       "72231e1f.openmfp.io",
		}

		newMgrFunc := kcp.ManagerFactory(opFlags)

		mgr, err := newMgrFunc(cfg, mgrOpts)
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		reconcilerOpts := kcp.ReconcilerOpts{
			Scheme:                 scheme,
			Config:                 cfg,
			OpenAPIDefinitionsPath: opFlags.OpenAPIdefinitionsPath,
		}

		newReconcilerFunc := kcp.ReconcilerFactory(opFlags)

		reconciler, err := newReconcilerFunc(reconcilerOpts)
		if err != nil {
			setupLog.Error(err, "unable to instantiate reconciler")
			os.Exit(1)
		}

		if err := reconciler.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller")
			os.Exit(1)
		}
		// +kubebuilder:scaffold:builder

		if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up health check")
			os.Exit(1)
		}
		if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
			setupLog.Error(err, "unable to set up ready check")
			os.Exit(1)
		}

		setupLog.Info("starting manager")
		signalHandler := ctrl.SetupSignalHandler()
		if err := mgr.Start(signalHandler); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	},
}
