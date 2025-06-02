package kcp

import (
	"bytes"
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/openmfp/golang-commons/logger"
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
	ErrGetVWConfig           = errors.New("unable to get virtual workspace config, check if your kcp cluster is running")
	ErrCreateHTTPClient      = errors.New("failed to create http client")
	ErrReadJSON              = errors.New("failed to read JSON from filesystem")
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

func NewReconciler(appCfg config.Config, opts ReconcilerOpts, restcfg *rest.Config,
	discoveryInterface discovery.DiscoveryInterface,
	preReconcileFunc func(cr *apischema.CRDResolver, io workspacefile.IOHandler) error,
	discoverFactory func(cfg *rest.Config) (*discoveryclient.FactoryProvider, error),
	log *logger.Logger,
) (CustomReconciler, error) {
	if !appCfg.EnableKcp {
		return newStandardReconciler(opts, discoveryInterface, preReconcileFunc, log)
	}

	return newKcpReconciler(opts, restcfg, discoverFactory, log)
}

func newStandardReconciler(
	opts ReconcilerOpts,
	discoveryInterface discovery.DiscoveryInterface,
	preReconcileFunc func(cr *apischema.CRDResolver, io workspacefile.IOHandler) error,
	log *logger.Logger,
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

	return controller.NewCRDReconciler(kubernetesClusterName, opts.Client, schemaResolver, ioHandler, log), nil
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
	io workspacefile.IOHandler,
) error {
	actualJSON, err := cr.Resolve()
	if err != nil {
		return errors.Join(ErrResolveSchema, err)
	}

	savedJSON, err := io.Read(kubernetesClusterName)
	if err != nil {
		if errors.Is(err, workspacefile.ErrNotFound) {
			return io.Write(actualJSON, kubernetesClusterName)
		}
		return errors.Join(ErrReadJSON, err)
	}

	if !bytes.Equal(actualJSON, savedJSON) {
		if err := io.Write(actualJSON, kubernetesClusterName); err != nil {
			return errors.Join(ErrWriteJSON, err)
		}
	}

	return nil
}

func newKcpReconciler(opts ReconcilerOpts, restcfg *rest.Config, newDiscoveryFactoryFunc func(cfg *rest.Config) (*discoveryclient.FactoryProvider, error), log *logger.Logger) (CustomReconciler, error) {
	ioHandler, err := workspacefile.NewIOHandler(opts.OpenAPIDefinitionsPath)
	if err != nil {
		return nil, errors.Join(ErrCreateIOHandler, err)
	}

	pr, err := clusterpath.NewResolver(opts.Config, opts.Scheme)
	if err != nil {
		return nil, errors.Join(ErrCreatePathResolver, err)
	}

	df, err := newDiscoveryFactoryFunc(restcfg)
	if err != nil {
		return nil, errors.Join(ErrCreateDiscoveryClient, err)
	}

	return controller.NewAPIBindingReconciler(
		ioHandler, df, apischema.NewResolver(), pr, log,
	), nil
}
