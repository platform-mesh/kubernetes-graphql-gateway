// Package context provides centralized context key definitions and helper functions
// for the gateway manager components.
package context

import (
	"context"

	"github.com/kcp-dev/logicalcluster/v3"
)

// Context key types - using unexported struct types for type safety and collision avoidance
type (
	tokenCtxKey        struct{}
	kcpWorkspaceCtxKey struct{}
	clusterNameCtxKey  struct{}
)

// Context key instances
var (
	tokenKey        = tokenCtxKey{}
	kcpWorkspaceKey = kcpWorkspaceCtxKey{}
	clusterNameKey  = clusterNameCtxKey{}
)

// Token context helpers

// TokenCtxKey is the exported type for token context keys
type TokenCtxKey = tokenCtxKey

// TokenKey returns the context key for storing authentication tokens
func TokenKey() tokenCtxKey {
	return tokenKey
}

// WithToken stores an authentication token in the context
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// TokenFromContext retrieves an authentication token from the context
func TokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(tokenKey).(string)
	return token, ok
}

// KCP Workspace context helpers

// KcpWorkspaceKey returns the context key for storing KCP workspace information
func KcpWorkspaceKey() kcpWorkspaceCtxKey {
	return kcpWorkspaceKey
}

// WithKcpWorkspace stores a KCP workspace identifier in the context
func WithKcpWorkspace(ctx context.Context, workspace string) context.Context {
	return context.WithValue(ctx, kcpWorkspaceKey, workspace)
}

// KcpWorkspaceFromContext retrieves a KCP workspace identifier from the context
func KcpWorkspaceFromContext(ctx context.Context) (string, bool) {
	workspace, ok := ctx.Value(kcpWorkspaceKey).(string)
	return workspace, ok
}

// Cluster Name context helpers

// ClusterNameKey returns the context key for storing logical cluster names
func ClusterNameKey() clusterNameCtxKey {
	return clusterNameKey
}

// WithClusterName stores a logical cluster name in the context
func WithClusterName(ctx context.Context, name logicalcluster.Name) context.Context {
	return context.WithValue(ctx, clusterNameKey, name)
}

// ClusterNameFromContext retrieves a logical cluster name from the context
func ClusterNameFromContext(ctx context.Context) (logicalcluster.Name, bool) {
	name, ok := ctx.Value(clusterNameKey).(logicalcluster.Name)
	return name, ok
}
