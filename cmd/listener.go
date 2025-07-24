package cmd

import (
	"context"
	"crypto/tls"
	"os"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcpcore "github.com/kcp-dev/kcp/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"
	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	gatewayv1alpha1 "github.com/openmfp/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/clusteraccess"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/reconciler/kcp"
)

var (
	scheme               = runtime.NewScheme()
	webhookServer        webhook.Server
	metricsServerOptions metricsserver.Options
)

var listenCmd = &cobra.Command{
	Use:     "listener",
	Example: "KUBECONFIG=<path to kubeconfig file> go run . listener",
	PreRun: func(cmd *cobra.Command, args []string) {
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

		webhookServer = webhook.NewServer(webhook.Options{
			TLSOpts: tlsOpts,
		})

		metricsServerOptions = metricsserver.Options{
			BindAddress:   defaultCfg.Metrics.BindAddress,
			SecureServing: defaultCfg.Metrics.Secure,
			TLSOpts:       tlsOpts,
		}

		if defaultCfg.Metrics.Secure {
			metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := ctrl.SetupSignalHandler()
		restCfg := ctrl.GetConfigOrDie()

		mgrOpts := ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsServerOptions,
			WebhookServer:          webhookServer,
			HealthProbeBindAddress: defaultCfg.HealthProbeBindAddress,
			LeaderElection:         defaultCfg.LeaderElection.Enabled,
			LeaderElectionID:       "72231e1f.openmfp.io",
		}

		clt, err := client.New(restCfg, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			log.Error().Err(err).Msg("failed to create client from config")
			os.Exit(1)
		}

		reconcilerOpts := reconciler.ReconcilerOpts{
			Scheme:                 scheme,
			Client:                 clt,
			Config:                 restCfg,
			ManagerOpts:            mgrOpts,
			OpenAPIDefinitionsPath: appCfg.OpenApiDefinitionsPath,
		}

		// Create the appropriate reconciler based on configuration
		var reconcilerInstance reconciler.CustomReconciler
		if appCfg.EnableKcp {
			kcpReconciler, err := kcp.NewKCPReconciler(appCfg, reconcilerOpts, log)
			if err != nil {
				log.Error().Err(err).Msg("unable to create KCP reconciler")
				os.Exit(1)
			}

			// Start virtual workspace watching if path is configured
			if appCfg.Listener.VirtualWorkspacesConfigPath != "" {
				go func() {
					if err := kcpReconciler.StartVirtualWorkspaceWatching(ctx, appCfg.Listener.VirtualWorkspacesConfigPath); err != nil {
						log.Error().Err(err).Msg("failed to start virtual workspace watching")
						os.Exit(1)
					}
				}()
			}

			reconcilerInstance = kcpReconciler
		} else {
			reconcilerInstance, err = clusteraccess.CreateMultiClusterReconciler(appCfg, reconcilerOpts, log)
			if err != nil {
				log.Error().Err(err).Msg("unable to create cluster access reconciler")
				os.Exit(1)
			}
		}

		// Setup reconciler with its own manager and start everything
		if err := startManagerWithReconciler(ctx, reconcilerInstance); err != nil {
			os.Exit(1)
		}
	},
}

// startManagerWithReconciler handles the common manager setup and start operations
func startManagerWithReconciler(ctx context.Context, reconciler reconciler.CustomReconciler) error {
	mgr := reconciler.GetManager()

	if err := reconciler.SetupWithManager(mgr); err != nil {
		log.Error().Err(err).Msg("unable to setup reconciler with manager")
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error().Err(err).Msg("unable to set up health check")
		return err
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error().Err(err).Msg("unable to set up ready check")
		return err
	}

	log.Info().Msg("starting manager")
	if err := mgr.Start(ctx); err != nil {
		log.Error().Err(err).Msg("problem running manager")
		return err
	}

	return nil
}
