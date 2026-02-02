package options

import (
	"fmt"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	providerkcp "github.com/platform-mesh/kubernetes-graphql-gateway/providers/kcp/options"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
)

type Options struct {
	Logs *logs.Options

	ProviderKcp *providerkcp.Options

	ExtraOptions
}

type ExtraOptions struct {
	// KubeConfig is the path to a kubeconfig. Only required if out-of-cluster
	KubeConfig string
	// Multicluster runtime provider
	Provider string
	// SchemasDir is the directory to store schema files.
	SchemasDir string
	// ResourceGVR is the GroupVersionResource which the reconciler will be watching
	ResourceGVR string
	// AnchorResource is the resource to watch for kubernetes provider
	// When a resource with this name exists, the controller will generate schema for the cluster
	AnchorResource string
	// ClusterMetadataFunc allows to provide cluster metadata for a given cluster name
	// when reconciling anchor namespaces.
	ClusterMetadataFunc v1alpha1.ClusterMetadataFunc
	// ClusterURLResolverFunc allows to provide cluster URL for a given cluster name
	ClusterURLResolverFunc v1alpha1.ClusterURLResolver
	// EnableHTTP2 indicates whether to enable HTTP/2 for controller-manager server
	EnableHTTP2 bool
	// MetricsBindAddress is the bind address for metrics server
	MetricsBindAddress string
	// MetricsSecureServe indicates whether to serve metrics over HTTPS
	MetricsSecureServe bool
}

type completedOptions struct {
	Logs *logs.Options

	// Provider specific options
	ProviderKcp *providerkcp.CompletedOptions

	ExtraOptions
}

type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	// Default to -v=2
	logs := logs.NewOptions()
	logs.Verbosity = logsv1.VerbosityLevel(2)

	opts := &Options{
		Logs:        logs,
		ProviderKcp: providerkcp.NewOptions(),

		ExtraOptions: ExtraOptions{
			Provider:               "kubernetes",
			SchemasDir:             "_output/schemas",
			AnchorResource:         "default",
			ResourceGVR:            "namespaces.v1",
			MetricsBindAddress:     "0",
			EnableHTTP2:            false,
			MetricsSecureServe:     false,
			ClusterURLResolverFunc: v1alpha1.DefaultClusterURLResolverFunc,
		},
	}
	return opts
}

var providerAliases = map[string]string{
	"kcp":        "kcp",
	"kubernetes": "kubernetes",
	"":           "kubernetes",
}

func (options *Options) AddFlags(fs *pflag.FlagSet) {
	logsv1.AddFlags(options.Logs, fs)
	options.ProviderKcp.AddFlags(fs)

	fs.StringVar(&options.KubeConfig, "kubeconfig", options.KubeConfig, "path to a kubeconfig. Only required if out-of-cluster")

	fs.StringVar(&options.Provider, "multicluster-runtime-provider", options.Provider,
		fmt.Sprintf("The multicluster runtime provider. Possible values are: %v", sets.List(sets.Set[string](sets.StringKeySet(providerAliases)))),
	)

	fs.StringVar(&options.SchemasDir, "schemas-dir", options.SchemasDir, "Directory to store schema files")

	fs.StringVar(&options.AnchorResource, "anchor-resource", options.AnchorResource, "Resource to watch as anchor for kubernetes provider (default: default)")
	fs.StringVar(&options.ResourceGVR, "reconciler-gvr", options.ResourceGVR, "The GroupVersionResource which the reconciler will be watching (default: namespaces.v1)")

	fs.BoolVar(&options.EnableHTTP2, "enable-http2", options.EnableHTTP2, "Enable HTTP/2 for controller-manager server")
	fs.StringVar(&options.MetricsBindAddress, "metrics-bind-address", options.MetricsBindAddress, "The address the metric endpoint binds to.")
	fs.BoolVar(&options.MetricsSecureServe, "metrics-secure-serve", options.MetricsSecureServe, "Serve metrics over HTTPS.")
}

func (options *Options) Complete() (*CompletedOptions, error) {
	co := &CompletedOptions{
		completedOptions: &completedOptions{
			Logs:         options.Logs,
			ExtraOptions: options.ExtraOptions,
		},
	}

	if options.Provider == "kcp" {
		opts, err := options.ProviderKcp.Complete()
		if err != nil {
			return nil, err
		}
		co.ProviderKcp = opts
		co.ClusterMetadataFunc = opts.GetClusterMetadataOverrideFunc()
		co.ClusterURLResolverFunc = opts.GetClusterURLResolverFunc()
	}
	return co, nil
}

func (options *CompletedOptions) Validate() error {
	provider := providerAliases[options.Provider]
	if provider == "" {
		return fmt.Errorf("unknown provider %q, must be one of %v", options.Provider, sets.List(sets.Set[string](sets.StringKeySet(providerAliases))))
	}
	options.Provider = provider

	gvr, gv := schema.ParseResourceArg(options.ResourceGVR)
	if gvr == nil && gv.Empty() {
		return fmt.Errorf("invalid reconciler-gvr %q", options.ResourceGVR)
	}

	return nil
}
