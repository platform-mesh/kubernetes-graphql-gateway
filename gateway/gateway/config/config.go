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

	// GRPCMaxRecvMsgSize is the maximum gRPC message size in bytes the gateway will accept.
	GRPCMaxRecvMsgSize int

	// GraphQL contains GraphQL-specific configuration
	GraphQL GraphQL

	// Limits contains DoS mitigation resource limits
	Limits Limits

	// TokenReviewCacheTTL is the duration to cache TokenReview results.
	TokenReviewCacheTTL time.Duration
}

// GraphQL holds GraphQL handler configuration.
type GraphQL struct {
	Pretty     bool
	Playground bool
	GraphiQL   bool
}

// Limits holds query validation limits enforced at the GraphQL layer.
// HTTP-level limits (body size, in-flight concurrency) live in http.ServerConfig.
type Limits struct {
	// MaxQueryDepth is the maximum allowed nesting depth for GraphQL queries.
	// 0 disables the limit.
	MaxQueryDepth int

	// MaxQueryComplexity is the maximum allowed complexity score for GraphQL queries.
	// Each field resolution counts as 1 point of complexity.
	// 0 disables the limit.
	MaxQueryComplexity int

	// MaxQueryBatchSize is the maximum number of queries allowed in a single batched request.
	// 0 disables the limit.
	MaxQueryBatchSize int
}
