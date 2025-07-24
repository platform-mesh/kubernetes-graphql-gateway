package kcp

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"

	"github.com/openmfp/golang-commons/logger"
	"github.com/openmfp/kubernetes-graphql-gateway/common/auth"
	"github.com/openmfp/kubernetes-graphql-gateway/listener/pkg/apischema"
)

// SchemaGenerationParams contains parameters for schema generation
type SchemaGenerationParams struct {
	ClusterPath     string
	DiscoveryClient discovery.DiscoveryInterface
	RESTMapper      meta.RESTMapper
	HostOverride    string // Optional: for virtual workspaces with custom URLs
}

// generateSchemaWithMetadata is a shared utility for schema generation
// Used by both regular APIBinding reconciliation and virtual workspace processing
func generateSchemaWithMetadata(
	params SchemaGenerationParams,
	apiSchemaResolver apischema.Resolver,
	log *logger.Logger,
) ([]byte, error) {
	log.Debug().Str("clusterPath", params.ClusterPath).Msg("starting API schema resolution")

	// Resolve current schema from API server
	rawSchema, err := apiSchemaResolver.Resolve(params.DiscoveryClient, params.RESTMapper)
	if err != nil {
		log.Error().Err(err).Msg("failed to resolve server JSON schema")
		return nil, fmt.Errorf("failed to resolve API schema: %w", err)
	}

	log.Debug().
		Str("clusterPath", params.ClusterPath).
		Int("schemaSize", len(rawSchema)).
		Msg("API schema resolved")

	// Inject KCP cluster metadata
	var schemaWithMetadata []byte
	if params.HostOverride != "" {
		// Virtual workspace with custom host
		schemaWithMetadata, err = auth.InjectKCPMetadataFromEnv(rawSchema, params.ClusterPath, log, params.HostOverride)
	} else {
		// Regular workspace using environment kubeconfig
		schemaWithMetadata, err = auth.InjectKCPMetadataFromEnv(rawSchema, params.ClusterPath, log)
	}

	if err != nil {
		log.Error().Err(err).Msg("failed to inject KCP cluster metadata")
		return nil, fmt.Errorf("failed to inject KCP cluster metadata: %w", err)
	}

	log.Debug().
		Str("clusterPath", params.ClusterPath).
		Int("finalSchemaSize", len(schemaWithMetadata)).
		Msg("schema generation completed with metadata injection")

	return schemaWithMetadata, nil
}
