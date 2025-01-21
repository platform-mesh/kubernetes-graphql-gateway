package controller

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"io/fs"

	"github.com/openmfp/crd-gql-gateway/listener/apischema"
	"github.com/openmfp/crd-gql-gateway/listener/discoveryclient"
	"github.com/openmfp/crd-gql-gateway/listener/workspacefile"

	kcpapis "github.com/kcp-dev/kcp/sdk/apis/apis/v1alpha1"
	kcptenancy "github.com/kcp-dev/kcp/sdk/apis/tenancy/v1alpha1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// APIBindingReconciler reconciles an APIBinding object
type APIBindingReconciler struct {
	io workspacefile.IOHandler
	df discoveryclient.Factory
	sc apischema.Resolver
}

func NewAPIBindingReconciler(
	io workspacefile.IOHandler,
	df discoveryclient.Factory,
	sc apischema.Resolver,
) *APIBindingReconciler {
	return &APIBindingReconciler{
		io: io,
		df: df,
		sc: sc,
	}
}

// +kubebuilder:rbac:groups=apis.kcp.io,resources=apibindings,verbs=get;list;watch
// +kubebuilder:rbac:groups=apis.kcp.io,resources=apibindings/status,verbs=get
// +kubebuilder:rbac:groups=tenancy.kcp.io,resources=workspaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=tenancy.kcp.io,resources=workspaces/status,verbs=get
func (r *APIBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	if strings.HasPrefix(req.ClusterName, "system") {
		return ctrl.Result{}, nil
	}

	logger := log.FromContext(ctx).WithValues("cluster", req.ClusterName)
	logger.Info("starting reconciliation...")

	dc, err := r.df.ClientForCluster(req.ClusterName)
	if err != nil {
		logger.Error(err, "failed to create discovery client for cluster")
		return ctrl.Result{}, err
	}

	savedJSON, err := r.io.Read(req.ClusterName)
	if errors.Is(err, fs.ErrNotExist) {
		actualJSON, err1 := r.sc.Resolve(dc)
		if err1 != nil {
			logger.Error(err1, "failed to resolve server JSON schema")
			return ctrl.Result{}, err1
		}
		if err = r.io.Write(actualJSON, req.ClusterName); err != nil {
			logger.Error(err, "failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error(err, "failed to read JSON from filesystem")
		return ctrl.Result{}, err
	}

	actualJSON, err := r.sc.Resolve(dc)
	if err != nil {
		logger.Error(err, "failed to resolve server JSON schema")
		return ctrl.Result{}, err
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		err = r.io.Write(actualJSON, req.ClusterName)
		if err != nil {
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
		Watches(&kcptenancy.Workspace{},
			handler.EnqueueRequestsFromMapFunc(clusterNameFromWorkspace)).
		Named("apibinding").
		Complete(r)
}
