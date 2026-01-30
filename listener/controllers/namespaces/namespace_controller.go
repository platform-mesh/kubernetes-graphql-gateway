/*
Copyright 2025 The Kube Bind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package namespaces

import (
	"context"
	"fmt"

	apisv1alpha2 "github.com/kcp-dev/sdk/apis/apis/v1alpha2"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/controllers/reconciler"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
)

const (
	controllerName = "namespace-schema-controller"
)

// NamespaceReconciler reconciles the anchor namespace to trigger schema generation
type NamespaceReconciler struct {
	manager         mcmanager.Manager
	opts            controller.TypedOptions[mcreconcile.Request]
	reconciler      *reconciler.Reconciler
	anchorNamespace string

	// Provider specific functions
	clusterMetadataFunc    v1alpha1.ClusterMetadataFunc
	clusterURLResolverFunc v1alpha1.ClusterURLResolver
}

// NewNamespaceReconciler returns a new NamespaceReconciler
func NewNamespaceReconciler(
	_ context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	ioHandler *workspacefile.FileHandler,
	schemaResolver apischema.Resolver,
	anchorNamespace string,
	clusterMetadataFunc v1alpha1.ClusterMetadataFunc,
	clusterURLResolverFunc v1alpha1.ClusterURLResolver,
) (*NamespaceReconciler, error) {
	r := &NamespaceReconciler{
		manager:         mgr,
		opts:            opts,
		reconciler:      reconciler.NewReconciler(ioHandler, schemaResolver),
		anchorNamespace: anchorNamespace,

		clusterMetadataFunc:    clusterMetadataFunc,
		clusterURLResolverFunc: clusterURLResolverFunc,
	}

	return r, nil
}

// Reconcile handles the namespace reconciliation
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling anchor namespace", "namespace", req.Name, "cluster", req.ClusterName)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	c := cl.GetClient()
	config := cl.GetConfig()

	config.Host, err = r.clusterURLResolverFunc(config.Host, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to resolve cluster URL: %w", err)
	}

	// If we are running in k8s mode, the cluster name might be empty.
	paths := []string{}
	if req.ClusterName == "" {
		paths = []string{"default"}
	} else {
		var apibindings apisv1alpha2.APIBindingList
		if err := c.List(ctx, &apibindings); err != nil {
			logger.Error(err, "Failed to list APIBindings")
			return ctrl.Result{}, fmt.Errorf("failed to list APIBindings: %w", err)
		}

		// There should be always just one APIBinding per workspace, but we loop for safety.
		for _, ab := range apibindings.Items {
			if ab.Annotations["kcp.io/path"] != "" {
				paths = append(paths, ab.Annotations["kcp.io/path"])
			}
		}
	}

	// Check if the anchor namespace exists
	ns := &corev1.Namespace{}
	if err := c.Get(ctx, client.ObjectKey{Name: r.anchorNamespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Anchor namespace not found, cleaning up schema", "namespace", r.anchorNamespace)
			// Delete the schema file if namespace is deleted
			if err := r.reconciler.Cleanup(paths); err != nil {
				logger.Error(err, "Failed to cleanup schema")
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get namespace: %w", err)
	}

	// This is plugable function to get cluster metadata for the given cluster name.
	var metadata *v1alpha1.ClusterMetadata
	if r.clusterMetadataFunc != nil {
		var err error
		metadata, err = r.clusterMetadataFunc(req.ClusterName)
		if err != nil {
			logger.Error(err, "Failed to get cluster metadata for namespace reconciliation", "cluster", req.ClusterName)
			return ctrl.Result{}, fmt.Errorf("failed to get cluster metadata for namespace reconciliation: %w", err)
		}
	}

	// Generate schema for the cluster
	if err := r.reconciler.Reconcile(ctx, paths, config, metadata); err != nil {
		logger.Error(err, "Failed to reconcile schema")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled schema for cluster")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *NamespaceReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	// Create a predicate to only watch the anchor namespace
	namespacePredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == r.anchorNamespace
	})

	return mcbuilder.ControllerManagedBy(mgr).
		For(&corev1.Namespace{}, mcbuilder.WithPredicates(namespacePredicate)).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}
