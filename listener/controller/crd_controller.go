package controller

import (
	"bytes"
	"context"
	"errors"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/workspacefile"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrReadJSON         = errors.New("failed to read JSON from filesystem")
	ErrResolveSchema    = errors.New("failed to resolve server JSON schema")
	ErrWriteJSON        = errors.New("failed to write JSON to filesystem")
	ErrGetReconciledObj = errors.New("failed to get reconciled object")
)

type CRDResolver interface {
	Resolve() ([]byte, error)
	ResolveApiSchema(crd *apiextensionsv1.CustomResourceDefinition) ([]byte, error)
}

// CRDReconciler reconciles a CustomResourceDefinition object
type CRDReconciler struct {
	ClusterName string
	client.Client
	CRDResolver
	io  workspacefile.IOHandler
	log *logger.Logger
}

func NewCRDReconciler(name string,
	clt client.Client,
	cr CRDResolver,
	io workspacefile.IOHandler,
	log *logger.Logger,
) *CRDReconciler {
	return &CRDReconciler{
		ClusterName: name,
		Client:      clt,
		CRDResolver: cr,
		io:          io,
		log:         log,
	}
}

func (r *CRDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.log.With().Str("cluster", r.ClusterName).Str("name", req.Name).Logger()
	logger.Info().Msg("starting reconciliation...")

	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := r.Client.Get(ctx, req.NamespacedName, crd)
	if apierrors.IsNotFound(err) {
		logger.Info().Msg("resource not found, updating schema...")
		return ctrl.Result{}, r.updateAPISchema()
	}
	if client.IgnoreNotFound(err) != nil {
		logger.Error().Err(err).Msg("failed to get reconciled object")
		return ctrl.Result{}, errors.Join(ErrGetReconciledObj, err)
	}

	return ctrl.Result{}, r.updateAPISchemaWith(crd)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CRDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiextensionsv1.CustomResourceDefinition{}).
		Named("CRD").
		Complete(r)
}

func (r *CRDReconciler) updateAPISchema() error {
	savedJSON, err := r.io.Read(r.ClusterName)
	if err != nil {
		return errors.Join(ErrReadJSON, err)
	}
	actualJSON, err := r.Resolve()
	if err != nil {
		return errors.Join(ErrResolveSchema, err)
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		if err := r.io.Write(actualJSON, r.ClusterName); err != nil {
			return errors.Join(ErrWriteJSON, err)
		}
	}
	return nil
}

func (r *CRDReconciler) updateAPISchemaWith(crd *apiextensionsv1.CustomResourceDefinition) error {
	savedJSON, err := r.io.Read(r.ClusterName)
	if err != nil {
		return errors.Join(ErrReadJSON, err)
	}
	actualJSON, err := r.ResolveApiSchema(crd)
	if err != nil {
		return errors.Join(ErrResolveSchema, err)
	}
	if !bytes.Equal(actualJSON, savedJSON) {
		if err := r.io.Write(actualJSON, r.ClusterName); err != nil {
			return errors.Join(ErrWriteJSON, err)
		}
	}
	return nil
}
