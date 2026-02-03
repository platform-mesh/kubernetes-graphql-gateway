package reconciler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/apischema"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// schemaGenerationParams contains parameters for schema generation
type schemaGenerationParams struct {
	ClusterPath     string
	DiscoveryClient discovery.DiscoveryInterface
	RESTMapper      meta.RESTMapper
	HostOverride    string // Optional: for virtual workspaces with custom URLs
}

// generateSchemaWithMetadata is a shared utility for schema generation
// Used by both regular APIBinding reconciliation and virtual workspace processing
func generateSchemaWithMetadata(
	ctx context.Context,
	params schemaGenerationParams,
	apiSchemaResolver apischema.Resolver,
	metadata *v1alpha1.ClusterMetadata,
) ([]byte, error) {
	logger := log.FromContext(ctx)

	logger.V(4).WithValues("clusterPath", params.ClusterPath).Info("starting API schema resolution")

	// Resolve current schema from API server
	rawSchema, err := apiSchemaResolver.Resolve(params.DiscoveryClient, params.RESTMapper)
	if err != nil {
		logger.Error(err, "failed to resolve server JSON schema")
		return nil, fmt.Errorf("failed to resolve API schema: %w", err)
	}

	logger.V(4).WithValues("clusterPath", params.ClusterPath, "schemaSize", len(rawSchema)).Info("API schema resolved")

	// Parse the existing schema JSON and inject cluster metadata if provided
	if metadata != nil {
		// TODO: This is ugly! Improve in future.
		var schemaJSON map[string]any
		if err := json.Unmarshal(rawSchema, &schemaJSON); err != nil {
			return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
		}

		data, err := json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cluster metadata: %w", err)
		}
		// marshal metadata into map[string]any
		var metadataMap map[string]any
		if err := json.Unmarshal(data, &metadataMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cluster metadata: %w", err)
		}

		// Inject the metadata into the schema
		schemaJSON["x-cluster-metadata"] = metadataMap
		return json.Marshal(schemaJSON)
	}

	return rawSchema, nil
}
