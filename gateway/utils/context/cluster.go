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

func SetToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// GetClusterFromCtx retrieves cluster from the request context
func GetClusterFromCtx(ctx context.Context) (string, bool) {
	return ctx.Value(clusterKey).(string), ctx.Value(clusterKey) != nil
}

// GetTokenFromCtx retrieves token from the request context
func GetTokenFromCtx(ctx context.Context) (string, bool) {
	return ctx.Value(tokenKey).(string), ctx.Value(tokenKey) != nil
}
