package reconciler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrCreateHTTPClient = errors.New("failed to create HTTP client")
	ErrCreateRESTMapper = errors.New("failed to create REST mapper")
)

type Reconciler struct {
	ioHandler      *workspacefile.FileHandler
	schemaResolver apischema.Resolver
}

func NewReconciler(
	ioHandler *workspacefile.FileHandler,
	schemaResolver apischema.Resolver,
) *Reconciler {
	return &Reconciler{
		ioHandler:      ioHandler,
		schemaResolver: schemaResolver,
	}
}

// Reconcile processes schema generation for the given schema paths and cluster config
// Paths are treated as aliased cluster paths for the same cluster config.
func (r *Reconciler) Reconcile(ctx context.Context, schemaPaths []string, cfg *rest.Config, metadata *v1alpha1.ClusterMetadata) error {
	logger := log.FromContext(ctx)

	logger.Info("Processing schema generation", "paths", schemaPaths)

	// Create discovery client for the host cluster
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		logger.Error(err, "Failed to create discovery client")
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Create REST mapper for the host clusters
	restMapper, err := r.restMapperFromConfig(cfg)
	if err != nil {
		logger.Error(err, "Failed to create REST mapper")
		return fmt.Errorf("failed to create REST mapper: %w", err)
	}

	// We store both representation and schema files for each cluster paths.
	for _, schemaPath := range schemaPaths {
		logger.Info("Generating schema", "path", schemaPath)
		params := schemaGenerationParams{
			ClusterPath:     schemaPath,
			DiscoveryClient: discoveryClient,
			RESTMapper:      restMapper,
		}

		currentSchema, err := generateSchemaWithMetadata(ctx, params, r.schemaResolver, metadata)
		if err != nil {
			logger.Error(err, "Failed to generate schema with metadata")
			return fmt.Errorf("failed to generate schema with metadata: %w", err)
		}

		// Read existing schema (if it exists)
		savedSchema, err := r.ioHandler.Read(schemaPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			logger.Error(err, "Failed to read existing schema file")
			return fmt.Errorf("failed to read existing schema: %w", err)
		}

		// Write if file doesn't exist or content has changed
		if errors.Is(err, fs.ErrNotExist) || !bytes.Equal(currentSchema, savedSchema) {
			if err := r.ioHandler.Write(currentSchema, schemaPath); err != nil {
				logger.Error(err, "Failed to write schema", "path", schemaPath)
				return fmt.Errorf("failed to write schema: %w", err)
			}
			logger.Info("Schema file updated", "path", schemaPath)
		} else {
			logger.Info("Schema unchanged, skipping write", "path", schemaPath)
		}
	}

	return nil
}

func (r *Reconciler) Cleanup(schemaPaths []string) error {
	for _, schemaPath := range schemaPaths {
		err := r.ioHandler.Delete(schemaPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to delete schema for path %q: %w", schemaPath, err)
		}
	}
	return nil
}

// restMapperFromConfig creates a REST mapper from a config
func (r *Reconciler) restMapperFromConfig(cfg *rest.Config) (meta.RESTMapper, error) {
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
