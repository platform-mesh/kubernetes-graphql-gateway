package context

import (
	"context"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// clusterKey is the context key for storing cluster information
const clusterKey contextKey = "cluster-key"

// tokenKey is the context key for storing authentication token
const tokenKey contextKey = "token-key"

// SetCluster sets cluster to the request context
func SetCluster(ctx context.Context, cluster string) context.Context {
	return context.WithValue(ctx, clusterKey, cluster)
}

// SetToken sets authentication token to the request context
func SetToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// GetClusterFromCtx retrieves cluster from the request context.
// Returns the cluster name and true if found, or empty string and false otherwise.
func GetClusterFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(clusterKey).(string)
	return v, ok
}

// GetTokenFromCtx retrieves token from the request context.
// Returns the token and true if found, or empty string and false otherwise.
func GetTokenFromCtx(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(tokenKey).(string)
	return v, ok
}
