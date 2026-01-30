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

package clusteraccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"
)

const (
	controllerName = "clusteraccess-schema-controller"
)

var (
	ErrCreateHTTPClient = errors.New("failed to create HTTP client")
	ErrCreateRESTMapper = errors.New("failed to create REST mapper")
)

// ClusterAccessReconciler reconciles ClusterAccess resources and generates schemas
type ClusterAccessReconciler struct {
	manager        mcmanager.Manager
	opts           controller.TypedOptions[mcreconcile.Request]
	ioHandler      *workspacefile.FileHandler
	schemaResolver apischema.Resolver
}

// NewClusterAccessReconciler returns a new ClusterAccessReconciler
func NewClusterAccessReconciler(
	_ context.Context,
	mgr mcmanager.Manager,
	opts controller.TypedOptions[mcreconcile.Request],
	ioHandler *workspacefile.FileHandler,
	schemaResolver apischema.Resolver,
) (*ClusterAccessReconciler, error) {
	r := &ClusterAccessReconciler{
		manager:        mgr,
		opts:           opts,
		ioHandler:      ioHandler,
		schemaResolver: schemaResolver,
	}

	return r, nil
}

// Reconcile handles the ClusterAccess reconciliation
func (r *ClusterAccessReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling ClusterAccess", "name", req.Name, "cluster", req.ClusterName)

	cl, err := r.manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get client for cluster %q: %w", req.ClusterName, err)
	}

	c := cl.GetClient()

	ca := &v1alpha1.ClusterAccess{}
	if err := c.Get(ctx, client.ObjectKey{Name: req.Name}, ca); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("ClusterAccess resource not found, cleaning up schema", "name", req.Name)
			// Delete the schema file if ClusterAccess is deleted
			// Try both possible paths (resource name and path field)
			name := req.Name
			if req.ClusterName != "" {
				name = fmt.Sprintf("%s-%s", req.ClusterName, name)
			}
			if err := r.ioHandler.Delete(name); err != nil {
				logger.Error(err, "Failed to cleanup schema")
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get ClusterAccess: %w", err)
	}

	// Determine cluster name/path for the schema file
	clusterName := ca.GetName()
	if ca.Spec.Path != "" {
		clusterName = ca.Spec.Path
	}
	if req.ClusterName != "" {
		clusterName = fmt.Sprintf("%s-%s", req.ClusterName, clusterName)
	}

	// Build target cluster config from ClusterAccess spec
	targetConfig, err := buildTargetClusterConfig(ctx, *ca, c)
	if err != nil {
		logger.Error(err, "Failed to build target cluster config", "clusterAccess", ca.Name)
		return ctrl.Result{}, err
	}

	// Create discovery client for target cluster
	targetDiscovery, err := discovery.NewDiscoveryClientForConfig(targetConfig)
	if err != nil {
		logger.Error(err, "Failed to create discovery client", "clusterAccess", ca.Name)
		return ctrl.Result{}, err
	}

	// Create REST mapper for target cluster
	targetRM, err := r.restMapperFromConfig(targetConfig)
	if err != nil {
		logger.Error(err, "Failed to create REST mapper", "clusterAccess", ca.Name)
		return ctrl.Result{}, err
	}

	// Create schema resolver for target cluster and resolve schema
	JSON, err := r.schemaResolver.Resolve(targetDiscovery, targetRM)
	if err != nil {
		logger.Error(err, "Failed to resolve schema", "clusterAccess", ca.Name)
		return ctrl.Result{}, err
	}

	// Inject cluster metadata into the schema
	schemaWithMetadata, err := injectClusterMetadata(ctx, JSON, *ca)
	if err != nil {
		logger.Error(err, "Failed to inject cluster metadata", "clusterAccess", ca.Name)
		return ctrl.Result{}, err
	}

	// Write schema to file
	if err := r.ioHandler.Write(schemaWithMetadata, clusterName); err != nil {
		logger.Error(err, "Failed to write schema", "clusterAccess", ca.Name)
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled schema for ClusterAccess", "name", ca.Name, "path", clusterName)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *ClusterAccessReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		For(&v1alpha1.ClusterAccess{}).
		WithOptions(r.opts).
		Named(controllerName).
		Complete(r)
}

// buildTargetClusterConfig extracts connection info from ClusterAccess and builds rest.Config
func buildTargetClusterConfig(ctx context.Context, clusterAccess v1alpha1.ClusterAccess, k8sClient client.Client) (*rest.Config, error) {
	spec := clusterAccess.Spec

	// Extract host (required)
	host := spec.Host
	if host == "" {
		return nil, errors.New("host field not found in ClusterAccess spec")
	}

	config, err := v1alpha1.BuildRestConfigFromClusterAccess(clusterAccess)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// injectClusterMetadata injects cluster metadata into schema JSON
// TODO: This is very unelegant, improve in future
func injectClusterMetadata(ctx context.Context, schemaData []byte, clusterAccess v1alpha1.ClusterAccess) ([]byte, error) {
	metadata, err := v1alpha1.BuildClusterMetadataFromClusterAccess(clusterAccess)
	if err != nil {
		return nil, fmt.Errorf("failed to build cluster metadata from ClusterAccess: %w", err)
	}

	// Parse the existing schema JSON
	var schemaJSON map[string]any
	if err := json.Unmarshal(schemaData, &schemaJSON); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cluster metadata: %w", err)
	}

	// Inject the metadata into the schema
	schemaJSON["x-cluster-metadata"] = data

	return json.Marshal(schemaJSON)
}

// restMapperFromConfig creates a REST mapper from a config
func (r *ClusterAccessReconciler) restMapperFromConfig(cfg *rest.Config) (meta.RESTMapper, error) {
	httpClt, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, errors.Join(ErrCreateHTTPClient, err)
	}
	rm, err := apiutil.NewDynamicRESTMapper(cfg, httpClt)
	if err != nil {
		return nil, errors.Join(ErrCreateRESTMapper, err)
	}

	return rm, nil
}
