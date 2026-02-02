package listener

import (
	"crypto/tls"
	"fmt"

	"github.com/platform-mesh/golang-commons/logger"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/options"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	kcpprovider "github.com/platform-mesh/kubernetes-graphql-gateway/providers/kcp"
	"github.com/rs/zerolog/log"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"

	kcpapisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	kcpapis "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	kcpcore "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	kcptenancy "github.com/kcp-dev/sdk/apis/tenancy/v1alpha1"

	"github.com/kcp-dev/multicluster-provider/apiexport"
)

type Config struct {
	Options *options.CompletedOptions

	Provider multicluster.Provider

	Manager mcmanager.Manager
	Scheme  *runtime.Scheme

	ClientConfig *rest.Config

	ReconcilerGVK schema.GroupVersionKind

	IOHandler      *workspacefile.FileHandler
	SchemaResolver apischema.Resolver

	// NamespaceReconcilerClusterMetadataFunc allows to provide cluster metadata for a given cluster name
	// when reconciling anchor namespaces.
	NamespaceReconcilerClusterMetadataFunc func(clusterName string) (*gatewayv1alpha1.ClusterMetadata, error)
}

func NewConfig(options *options.CompletedOptions) (*Config, error) {
	config := &Config{
		Options: options,
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = options.KubeConfig

	var err error
	config.ClientConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
	if err != nil {
		return nil, err
	}

	config.ClientConfig = rest.CopyConfig(config.ClientConfig)
	config.ClientConfig = rest.AddUserAgent(config.ClientConfig, "kubernetes-graphql-gateway-listener")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding client-go scheme: %w", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding apiextensions scheme: %w", err)
	}
	if err := gatewayv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("error adding kubebind scheme: %w", err)
	}

	config.Scheme = scheme

	switch options.Provider {
	case "kcp":
		if options.ProviderKcp == nil {
			return nil, fmt.Errorf("kcp provider options must be provided when provider is kcp")
		}
		if err := kcpapisv1alpha1.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("error adding apis scheme: %w", err)
		}
		if err := kcpapis.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("error adding apis scheme: %w", err)
		}
		if err := kcpcore.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("error adding core scheme: %w", err)
		}
		if err := kcptenancy.AddToScheme(scheme); err != nil {
			return nil, fmt.Errorf("error adding tenancy scheme: %w", err)
		}

		provider, err := kcpprovider.New(config.ClientConfig, options.ProviderKcp.APIExportEndpointSliceName, apiexport.Options{
			Scheme: scheme,
		})
		if err != nil {
			return nil, fmt.Errorf("error setting up kcp provider: %w", err)
		}

		config.Provider = provider
		config.NamespaceReconcilerClusterMetadataFunc = options.ProviderKcp.GetClusterMetadataOverrideFunc()
	default:
		config.Provider = nil
	}

	var tlsOpts []func(*tls.Config)
	if !options.EnableHTTP2 {
		disableHTTP2 := func(c *tls.Config) {
			log.Info().Msg("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		}
		tlsOpts = []func(c *tls.Config){disableHTTP2}
	}

	opts := ctrl.Options{
		Controller: ctrlconfig.Controller{},
		Metrics: metricsserver.Options{
			BindAddress:   options.MetricsBindAddress,
			SecureServing: options.MetricsSecureServe,
			TLSOpts:       tlsOpts,
		},
		Scheme:           scheme,
		LeaderElectionID: "72231e1f.platform-mesh.io",
	}
	if options.MetricsSecureServe {
		opts.Metrics.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	manager, err := mcmanager.New(config.ClientConfig, config.Provider, opts)
	if err != nil {
		return nil, fmt.Errorf("error setting up controller manager: %w", err)
	}

	config.Manager = manager

	// Initialize FileHandler for schema storage
	ioHandler, err := workspacefile.NewIOHandler(options.SchemasDir)
	if err != nil {
		return nil, fmt.Errorf("error creating IO handler: %w", err)
	}
	config.IOHandler = ioHandler

	// Initialize schema resolver
	// TODO: Move to context based logger.
	log, err := logger.New(logger.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("error creating logger: %w", err)
	}
	config.SchemaResolver = apischema.NewResolver(log)

	return config, nil
}
