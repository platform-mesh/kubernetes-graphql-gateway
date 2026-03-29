package config

import "time"

// Gateway holds the complete gateway service configuration.
type Gateway struct {
	// SchemaHandler specifies which watcher to use ("file" or "grpc")
	SchemaHandler string

	// SchemaDirectory is the directory to watch when SchemaHandler is "file"
	SchemaDirectory string

	// GRPCAddress is the gRPC server address when SchemaHandler is "grpc"
	GRPCAddress string

	// GraphQL contains GraphQL-specific configuration
	GraphQL GraphQL

	// TokenReviewCacheTTL is the duration to cache TokenReview results.
	TokenReviewCacheTTL time.Duration
}

// GraphQL holds GraphQL handler configuration.
type GraphQL struct {
	Pretty     bool
	Playground bool
	GraphiQL   bool
}
