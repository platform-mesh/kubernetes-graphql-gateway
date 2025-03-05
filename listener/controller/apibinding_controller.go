package controller

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"io/fs"

	"github.com/openmfp/crd-gql-gateway/listener/apischema"
	"github.com/openmfp/crd-gql-gateway/listener/clusterpath"
	"github.com/openmfp/crd-gql-gateway/listener/discoveryclient"
	"github.com/openmfp/crd-gql-gateway/listener/workspacefile"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// APIBindingReconciler reconciles an APIBinding object
type APIBindingReconciler struct {
	io workspacefile.IOHandler
	df discoveryclient.Factory
	sc apischema.Resolver
	pr *clusterpath.Resolver
}

func NewAPIBindingReconciler(
	io workspacefile.IOHandler,
	df discoveryclient.Factory,
	sc apischema.Resolver,
	pr *clusterpath.Resolver,
) *APIBindingReconciler {
	return &APIBindingReconciler{
		io: io,
		df: df,
		sc: sc,
		pr: pr,
	}
}

func (r *APIBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// ignore system workspaces (e.g. system:shard)
	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx)
	clusterPath, err := r.pr.ResolverFunc(req.ClusterName, r.pr.Config, r.pr.Scheme)
	if err != nil {
		logger.Error(err, "failed to get cluster path", "cluster", req.ClusterName)
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("cluster", clusterPath)
	logger.Info("starting reconciliation...")

	dc, err := r.df.ClientForCluster(clusterPath)
	if err != nil {
		logger.Error(err, "failed to create discovery client for cluster")
		return ctrl.Result{}, err
	}

	rm, err := r.df.RestMapperForCluster(clusterPath)
	if err != nil {
		logger.Error(err, "failed to create rest mapper for cluster")
		return ctrl.Result{}, err
	}

	savedJSON, err := r.io.Read(clusterPath)
	if errors.Is(err, fs.ErrNotExist) {
		actualJSON, err1 := r.sc.Resolve(dc, rm)
		if err1 != nil {
			logger.Error(err1, "failed to resolve server JSON schema")
			return ctrl.Result{}, err1
		}
		if err := r.io.Write(actualJSON, clusterPath); err != nil {
			logger.Error(err, "failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error(err, "failed to read JSON from filesystem")
		return ctrl.Result{}, err
	}

	actualJSON, err := r.sc.Resolve(dc, rm)
	if err != nil {
		logger.Error(err, "failed to resolve server JSON schema")
		return ctrl.Result{}, err
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		if err := r.io.Write(actualJSON, clusterPath); err != nil {
			logger.Error(err, "failed to write JSON to filesystem")
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
