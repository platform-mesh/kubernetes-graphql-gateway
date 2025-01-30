package controller

import (
	"bytes"
	"context"
	"errors"

	"io/fs"

	"github.com/openmfp/crd-gql-gateway/listener/apischema"
	"github.com/openmfp/crd-gql-gateway/listener/workspacefile"
	"k8s.io/client-go/discovery"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CRDReconciler reconciles a CustomResourceDefinition object
type CRDReconciler struct {
	ClusterName string
	client.Client
	*discovery.DiscoveryClient
	io workspacefile.IOHandler
	sc apischema.Resolver
}

func NewCRDReconciler(name string,
	clt client.Client,
	dc *discovery.DiscoveryClient,
	io workspacefile.IOHandler,
	sc apischema.Resolver,
) *CRDReconciler {
	return &CRDReconciler{
		ClusterName:     name,
		Client:          clt,
		DiscoveryClient: dc,
		io:              io,
		sc:              sc,
	}
}

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinition,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinition/status,verbs=get
func (r *CRDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx).WithValues("cluster", r.ClusterName).WithName(req.Name)
	logger.Info("starting reconciliation...")

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Client.Get(ctx, req.NamespacedName, crd); client.IgnoreNotFound(err) != nil {
		logger.Error(err, "failed to get reconciled object")
		return ctrl.Result{}, err
	}

	savedJSON, err := r.io.Read(r.ClusterName)
	if errors.Is(err, fs.ErrNotExist) {
		actualJSON, err1 := r.sc.Resolve(r.DiscoveryClient)
		if err1 != nil {
			logger.Error(err1, "failed to resolve server JSON schema")
			return ctrl.Result{}, err1
		}
		if err := r.io.Write(actualJSON, r.ClusterName); err != nil {
			logger.Error(err, "failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err != nil {
		logger.Error(err, "failed to read JSON from filesystem")
		return ctrl.Result{}, err
	}

	actualJSON, err := r.sc.Resolve(r.DiscoveryClient)
	if err != nil {
		logger.Error(err, "failed to resolve server JSON schema")
		return ctrl.Result{}, err
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		if err := r.io.Write(actualJSON, r.ClusterName); err != nil {
			logger.Error(err, "failed to write JSON to filesystem")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CRDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiextensionsv1.CustomResourceDefinition{}).
		Named("CRD").
		Complete(r)
}
