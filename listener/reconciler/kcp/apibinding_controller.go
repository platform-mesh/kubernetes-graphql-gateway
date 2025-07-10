package kcp

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"strings"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// APIBindingReconciler reconciles an APIBinding object
type APIBindingReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	RestConfig          *rest.Config
	IOHandler           workspacefile.IOHandler
	DiscoveryFactory    DiscoveryFactory
	APISchemaResolver   apischema.Resolver
	ClusterPathResolver ClusterPathResolver
	Log                 *logger.Logger
}

func (r *APIBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ignore system workspaces (e.g. system:shard)
	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := r.Log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()

	clusterClt, err := r.ClusterPathResolver.ClientForCluster(req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get cluster client")
		return ctrl.Result{}, err
	}

	clusterPath, err := PathForCluster(req.ClusterName, clusterClt)
	if err != nil {
		if errors.Is(err, ErrClusterIsDeleted) {
			logger.Info().Msg("cluster is deleted, triggering cleanup")
			if err = r.IOHandler.Delete(clusterPath); err != nil {
				logger.Error().Err(err).Msg("failed to delete workspace file after cluster deletion")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		logger.Error().Err(err).Msg("failed to get cluster path")
		return ctrl.Result{}, err
	}

	logger = logger.With().Str("clusterPath", clusterPath).Logger()
	logger.Info().Msg("starting reconciliation...")

	dc, err := r.DiscoveryFactory.ClientForCluster(clusterPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create discovery client for cluster")
		return ctrl.Result{}, err
	}

	rm, err := r.DiscoveryFactory.RestMapperForCluster(clusterPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create rest mapper for cluster")
		return ctrl.Result{}, err
	}

	savedJSON, err := r.IOHandler.Read(clusterPath)
	if errors.Is(err, fs.ErrNotExist) {
		actualJSON, err1 := r.APISchemaResolver.Resolve(dc, rm)
		if err1 != nil {
			logger.Error().Err(err1).Msg("failed to resolve server JSON schema")
			return ctrl.Result{}, err1
		}
		if err := r.IOHandler.Write(actualJSON, clusterPath); err != nil {
			logger.Error().Err(err).Msg("failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error().Err(err).Msg("failed to read JSON from filesystem")
		return ctrl.Result{}, err
	}

	actualJSON, err := r.APISchemaResolver.Resolve(dc, rm)
	if err != nil {
		logger.Error().Err(err).Msg("failed to resolve server JSON schema")
		return ctrl.Result{}, err
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		if err := r.IOHandler.Write(actualJSON, clusterPath); err != nil {
			logger.Error().Err(err).Msg("failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *APIBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kcpapis.APIBinding{}).
		Complete(r)
}
