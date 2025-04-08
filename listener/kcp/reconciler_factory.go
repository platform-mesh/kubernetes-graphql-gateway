package kcp

import (
	"context"
	"errors"
	"github.com/openmfp/golang-commons/logger"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/openmfp/kubernetes-graphql-gateway/common/config"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/clusterpath"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/controller"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/discoveryclient"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile"
)

const (
	kubernetesClusterName = "kubernetes" // is used as a name for the schema file in case of a standard k8s cluster.
)

var (
	ErrCreateDiscoveryClient = errors.New("failed to create discovery client")
	ErrCreateIOHandler       = errors.New("failed to create IO Handler")
	ErrCreateRestMapper      = errors.New("failed to create rest mapper")
	ErrGenerateSchema        = errors.New("failed to generate OpenAPI Schema")
	ErrResolveSchema         = errors.New("failed to resolve server JSON schema")
	ErrWriteJSON             = errors.New("failed to write JSON to filesystem")
	ErrCreatePathResolver    = errors.New("failed to create cluster path resolver")
	ErrGetVWConfig           = errors.New("unable to get virtual workspace config")
	ErrCreateHTTPClient      = errors.New("failed to create http client")
)

type CustomReconciler interface {
	Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
	SetupWithManager(mgr ctrl.Manager) error
}

type ReconcilerOpts struct {
	*rest.Config
	*runtime.Scheme
	client.Client
	OpenAPIDefinitionsPath string
}

func NewReconciler(
	ctx context.Context,
	log *logger.Logger,
	appCfg config.Config,
	opts ReconcilerOpts,
	discoveryInterface discovery.DiscoveryInterface,
	preReconcileFunc func(cr *apischema.CRDResolver, io *workspacefile.IOHandler) error,
	discoverFactory func(cfg *rest.Config) (*discoveryclient.Factory, error),
) (CustomReconciler, error) {
	if !appCfg.EnableKcp {
		return newStandardReconciler(opts, discoveryInterface, preReconcileFunc)
	}

	return newKcpReconciler(ctx, log, appCfg, opts, discoverFactory)
}

func newStandardReconciler(
	opts ReconcilerOpts,
	discoveryInterface discovery.DiscoveryInterface,
	preReconcileFunc func(cr *apischema.CRDResolver, io *workspacefile.IOHandler) error,
) (CustomReconciler, error) {
	ioHandler, err := workspacefile.NewIOHandler(opts.OpenAPIDefinitionsPath)
	if err != nil {
		return nil, errors.Join(ErrCreateIOHandler, err)
	}

	rm, err := restMapperFromConfig(opts.Config)
	if err != nil {
		return nil, err
	}

	schemaResolver := &apischema.CRDResolver{
		DiscoveryInterface: discoveryInterface,
		RESTMapper:         rm,
	}

	if err = preReconcileFunc(schemaResolver, ioHandler); err != nil {
		return nil, errors.Join(ErrGenerateSchema, err)
	}

	return controller.NewCRDReconciler(kubernetesClusterName, opts.Client, schemaResolver, ioHandler), nil
}

func restMapperFromConfig(cfg *rest.Config) (meta.RESTMapper, error) {
	httpClt, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, errors.Join(ErrCreateHTTPClient, err)
	}
	rm, err := apiutil.NewDynamicRESTMapper(cfg, httpClt)
	if err != nil {
		return nil, errors.Join(ErrCreateRestMapper, err)
	}

	return rm, nil
}

func PreReconcile(
	cr *apischema.CRDResolver,
	io *workspacefile.IOHandler,
) error {
	JSON, err := cr.Resolve()
	if err != nil {
		return errors.Join(ErrResolveSchema, err)
	}
	if err := io.Write(JSON, kubernetesClusterName); err != nil {
		return errors.Join(ErrWriteJSON, err)
	}

	return nil
}

func newKcpReconciler(
	ctx context.Context,
	log *logger.Logger,
	appCfg config.Config,
	opts ReconcilerOpts,
	newDiscoveryFactoryFunc func(cfg *rest.Config) (*discoveryclient.Factory, error),
) (CustomReconciler, error) {
	ioHandler, err := workspacefile.NewIOHandler(opts.OpenAPIDefinitionsPath)
	if err != nil {
		return nil, errors.Join(ErrCreateIOHandler, err)
	}

	pr, err := clusterpath.NewResolver(opts.Config, opts.Scheme)
	if err != nil {
		return nil, errors.Join(ErrCreatePathResolver, err)
	}

	virtualWorkspaceCfg, err := virtualWorkspaceConfigFromCfg(ctx, log, appCfg, opts.Config, opts.Client)
	if err != nil {
		return nil, errors.Join(ErrGetVWConfig, err)
	}

	df, err := newDiscoveryFactoryFunc(virtualWorkspaceCfg)
	if err != nil {
		return nil, errors.Join(ErrCreateDiscoveryClient, err)
	}

	return controller.NewAPIBindingReconciler(
		ioHandler, df, apischema.NewResolver(), pr,
	), nil
}
