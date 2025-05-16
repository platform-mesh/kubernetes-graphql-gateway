package controller

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"io/fs"

	"github.com/openmfp/kubernetes-graphql-gateway/listener/apischema"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/clusterpath"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/discoveryclient"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"

	"github.com/openmfp/golang-commons/logger"
	ctrl "sigs.k8s.io/controller-runtime"
)

// APIBindingReconciler reconciles an APIBinding object
type APIBindingReconciler struct {
	ioHandler           workspacefile.IOHandler
	discoveryFactory    discoveryclient.Factory
	apiSchemaResolver   apischema.Resolver
	clusterPathResolver clusterpath.Resolver
	log                 *logger.Logger
}

func NewAPIBindingReconciler(
	ioHandler workspacefile.IOHandler,
	discoveryFactory discoveryclient.Factory,
	apiSchemaResolver apischema.Resolver,
	clusterPathResolver clusterpath.Resolver,
	log *logger.Logger,
) *APIBindingReconciler {
	return &APIBindingReconciler{
		ioHandler:           ioHandler,
		discoveryFactory:    discoveryFactory,
		apiSchemaResolver:   apiSchemaResolver,
		clusterPathResolver: clusterPathResolver,
		log:                 log,
	}
}

func (r *APIBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ignore system workspaces (e.g. system:shard)
	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := r.log.With().Str("cluster", req.ClusterName).Str("name", req.Name).Logger()
	clusterClt, err := r.clusterPathResolver.ClientForCluster(req.ClusterName)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get cluster client")
		return ctrl.Result{}, err
	}
	clusterPath, err := clusterpath.PathForCluster(req.ClusterName, clusterClt)
	if err != nil {
		if errors.Is(err, clusterpath.ErrClusterIsDeleted) {
			logger.Info().Msg("cluster is deleted, triggering cleanup")
			if err = r.ioHandler.Delete(clusterPath); err != nil {
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

	dc, err := r.discoveryFactory.ClientForCluster(clusterPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create discovery client for cluster")
		return ctrl.Result{}, err
	}

	rm, err := r.discoveryFactory.RestMapperForCluster(clusterPath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create rest mapper for cluster")
		return ctrl.Result{}, err
	}

	savedJSON, err := r.ioHandler.Read(clusterPath)
	if errors.Is(err, fs.ErrNotExist) {
		actualJSON, err1 := r.apiSchemaResolver.Resolve(dc, rm)
		if err1 != nil {
			logger.Error().Err(err1).Msg("failed to resolve server JSON schema")
			return ctrl.Result{}, err1
		}
		if err := r.ioHandler.Write(actualJSON, clusterPath); err != nil {
			logger.Error().Err(err).Msg("failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error().Err(err).Msg("failed to read JSON from filesystem")
		return ctrl.Result{}, err
	}

	actualJSON, err := r.apiSchemaResolver.Resolve(dc, rm)
	if err != nil {
		logger.Error().Err(err).Msg("failed to resolve server JSON schema")
		return ctrl.Result{}, err
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		if err := r.ioHandler.Write(actualJSON, clusterPath); err != nil {
			logger.Error().Err(err).Msg("failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kcpapis.APIBinding{}).
		Named("apibinding").
		Complete(r)
}
