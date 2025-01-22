package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"net/url"
	"os"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/openmfp/crd-gql-gateway/listener/clusterpath"
	"github.com/openmfp/crd-gql-gateway/listener/flags"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kcpctrl "sigs.k8s.io/controller-runtime/pkg/kcp"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/openmfp/crd-gql-gateway/listener/apischema"
	"github.com/openmfp/crd-gql-gateway/listener/controller"
	"github.com/openmfp/crd-gql-gateway/listener/discoveryclient"
	"github.com/openmfp/crd-gql-gateway/listener/workspacefile"
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
	Example: "KUBECONFIG=.kcp/admin.kubeconfig go run . listen",
	PreRun: func(cmd *cobra.Command, args []string) {
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))

		utilruntime.Must(kcpapis.AddToScheme(scheme))
		utilruntime.Must(kcpcore.AddToScheme(scheme))
		utilruntime.Must(kcptenancy.AddToScheme(scheme))
		utilruntime.Must(apiextensions.AddToScheme(scheme))
		// +kubebuilder:scaffold:scheme

		var err error
		opFlags, err = flags.NewFromEnv()
		if err != nil {
			log.Fatal().Err(err).Msg("Error getting app restCfg, exiting")
		}
		opts := zap.Options{
			Development: true,
		}

		ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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
		cfgURL, err := url.Parse(cfg.Host)
		if err != nil {
			setupLog.Error(err, "failed to parse config Host")
			os.Exit(1)
		}
		clt, err := client.New(cfg, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			setupLog.Error(err, "failed to create client from config")
			os.Exit(1)
		}
		tenancyAPIExport := &kcpapis.APIExport{}
		err = clt.Get(context.TODO(), client.ObjectKey{Name: kcptenancy.SchemeGroupVersion.Group}, tenancyAPIExport)
		if err != nil {
			setupLog.Error(err, "failed to get tenancy APIExport")
			os.Exit(1)
		}
		virtualWorkspaces := tenancyAPIExport.Status.VirtualWorkspaces // nolint: staticcheck
		if len(virtualWorkspaces) == 0 {
			err := errors.New("empty virtual workspace list")
			setupLog.Error(err, "failed to get at least one virtual workspace")
			os.Exit(1)
		}
		vwCFGURL, err := url.Parse(virtualWorkspaces[0].URL)
		if err != nil {
			setupLog.Error(err, "failed to parse virtual workspace config URL")
			os.Exit(1)
		}
		cfgURL.Path = vwCFGURL.Path
		virtualWorkspaceCfg := rest.CopyConfig(cfg)
		virtualWorkspaceCfg.Host = cfgURL.String()

		mgr, err := kcpctrl.NewClusterAwareManager(virtualWorkspaceCfg, ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsServerOptions,
			WebhookServer:          webhookServer,
			HealthProbeBindAddress: opFlags.ProbeAddr,
			LeaderElection:         opFlags.EnableLeaderElection,
			LeaderElectionID:       "72231e1f.openmfp.io",
		})
		if err != nil {
			setupLog.Error(err, "unable to start manager")
			os.Exit(1)
		}

		ioHandler, err := workspacefile.NewIOHandler(opFlags.OpenAPIdefinitionsPath)
		if err != nil {
			setupLog.Error(err, "failed to create IO Handler")
			os.Exit(1)
		}

		df, err := discoveryclient.NewFactory(virtualWorkspaceCfg)
		if err != nil {
			setupLog.Error(err, "failed to create Discovery client factory")
			os.Exit(1)
		}

		reconciler := controller.NewAPIBindingReconciler(
			ioHandler, df, apischema.NewResolver(), &clusterpath.Resolver{
				Scheme:       mgr.GetScheme(),
				Config:       cfg,
				ResolverFunc: clusterpath.Resolve,
			},
		)

		err = reconciler.SetupWithManager(mgr)
		if err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Workspace")
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
		err = mgr.Start(signalHandler)
		if err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	},
}
