package context_test

import (
	"context"
	"testing"

	"github.com/kcp-dev/logicalcluster/v3"
	"github.com/stretchr/testify/assert"

	ctxkeys "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/manager/context"
)

func TestTokenContextHelpers(t *testing.T) {
	t.Run("TokenKey", func(t *testing.T) {
		key1 := ctxkeys.TokenKey()
		key2 := ctxkeys.TokenKey()
		assert.Equal(t, key1, key2, "TokenKey should return consistent values")
	})

	t.Run("WithToken_and_TokenFromContext", func(t *testing.T) {
		ctx := context.Background()
		token := "test-token-123"

		// Store token in context
		ctxWithToken := ctxkeys.WithToken(ctx, token)
		assert.NotEqual(t, ctx, ctxWithToken, "Context should be different after adding token")

		// Retrieve token from context
		retrievedToken, ok := ctxkeys.TokenFromContext(ctxWithToken)
		assert.True(t, ok, "Token should be found in context")
		assert.Equal(t, token, retrievedToken, "Retrieved token should match stored token")
	})

	t.Run("TokenFromContext_empty_context", func(t *testing.T) {
		ctx := context.Background()

		token, ok := ctxkeys.TokenFromContext(ctx)
		assert.False(t, ok, "Token should not be found in empty context")
		assert.Empty(t, token, "Token should be empty when not found")
	})

	t.Run("TokenFromContext_wrong_type", func(t *testing.T) {
		ctx := context.Background()
		// Store a non-string value using the token key
		ctxWithWrongType := context.WithValue(ctx, ctxkeys.TokenKey(), 123)

		token, ok := ctxkeys.TokenFromContext(ctxWithWrongType)
		assert.False(t, ok, "Token should not be found when wrong type is stored")
		assert.Empty(t, token, "Token should be empty when wrong type is stored")
	})

	t.Run("WithToken_empty_string", func(t *testing.T) {
		ctx := context.Background()
		emptyToken := ""

		ctxWithToken := ctxkeys.WithToken(ctx, emptyToken)
		retrievedToken, ok := ctxkeys.TokenFromContext(ctxWithToken)
		assert.True(t, ok, "Empty token should still be found in context")
		assert.Equal(t, emptyToken, retrievedToken, "Empty token should be retrievable")
	})
}

func TestKcpWorkspaceContextHelpers(t *testing.T) {
	t.Run("KcpWorkspaceKey", func(t *testing.T) {
		key1 := ctxkeys.KcpWorkspaceKey()
		key2 := ctxkeys.KcpWorkspaceKey()
		assert.Equal(t, key1, key2, "KcpWorkspaceKey should return consistent values")
	})

	t.Run("WithKcpWorkspace_and_KcpWorkspaceFromContext", func(t *testing.T) {
		ctx := context.Background()
		workspace := "root:orgs:default"

		// Store workspace in context
		ctxWithWorkspace := ctxkeys.WithKcpWorkspace(ctx, workspace)
		assert.NotEqual(t, ctx, ctxWithWorkspace, "Context should be different after adding workspace")

		// Retrieve workspace from context
		retrievedWorkspace, ok := ctxkeys.KcpWorkspaceFromContext(ctxWithWorkspace)
		assert.True(t, ok, "Workspace should be found in context")
		assert.Equal(t, workspace, retrievedWorkspace, "Retrieved workspace should match stored workspace")
	})

	t.Run("KcpWorkspaceFromContext_empty_context", func(t *testing.T) {
		ctx := context.Background()

		workspace, ok := ctxkeys.KcpWorkspaceFromContext(ctx)
		assert.False(t, ok, "Workspace should not be found in empty context")
		assert.Empty(t, workspace, "Workspace should be empty when not found")
	})

	t.Run("KcpWorkspaceFromContext_wrong_type", func(t *testing.T) {
		ctx := context.Background()
		// Store a non-string value using the workspace key
		ctxWithWrongType := context.WithValue(ctx, ctxkeys.KcpWorkspaceKey(), 456)

		workspace, ok := ctxkeys.KcpWorkspaceFromContext(ctxWithWrongType)
		assert.False(t, ok, "Workspace should not be found when wrong type is stored")
		assert.Empty(t, workspace, "Workspace should be empty when wrong type is stored")
	})

	t.Run("WithKcpWorkspace_complex_path", func(t *testing.T) {
		ctx := context.Background()
		complexWorkspace := "root:orgs:company:team:project"

		ctxWithWorkspace := ctxkeys.WithKcpWorkspace(ctx, complexWorkspace)
		retrievedWorkspace, ok := ctxkeys.KcpWorkspaceFromContext(ctxWithWorkspace)
		assert.True(t, ok, "Complex workspace should be found in context")
		assert.Equal(t, complexWorkspace, retrievedWorkspace, "Complex workspace should be retrievable")
	})
}

func TestClusterNameContextHelpers(t *testing.T) {
	t.Run("ClusterNameKey", func(t *testing.T) {
		key1 := ctxkeys.ClusterNameKey()
		key2 := ctxkeys.ClusterNameKey()
		assert.Equal(t, key1, key2, "ClusterNameKey should return consistent values")
	})

	t.Run("WithClusterName_and_ClusterNameFromContext", func(t *testing.T) {
		ctx := context.Background()
		clusterName := logicalcluster.Name("test-cluster")

		// Store cluster name in context
		ctxWithClusterName := ctxkeys.WithClusterName(ctx, clusterName)
		assert.NotEqual(t, ctx, ctxWithClusterName, "Context should be different after adding cluster name")

		// Retrieve cluster name from context
		retrievedClusterName, ok := ctxkeys.ClusterNameFromContext(ctxWithClusterName)
		assert.True(t, ok, "Cluster name should be found in context")
		assert.Equal(t, clusterName, retrievedClusterName, "Retrieved cluster name should match stored cluster name")
	})

	t.Run("ClusterNameFromContext_empty_context", func(t *testing.T) {
		ctx := context.Background()

		clusterName, ok := ctxkeys.ClusterNameFromContext(ctx)
		assert.False(t, ok, "Cluster name should not be found in empty context")
		assert.Equal(t, logicalcluster.Name(""), clusterName, "Cluster name should be empty when not found")
	})

	t.Run("ClusterNameFromContext_wrong_type", func(t *testing.T) {
		ctx := context.Background()
		// Store a non-logicalcluster.Name value using the cluster name key
		ctxWithWrongType := context.WithValue(ctx, ctxkeys.ClusterNameKey(), "wrong-type")

		clusterName, ok := ctxkeys.ClusterNameFromContext(ctxWithWrongType)
		assert.False(t, ok, "Cluster name should not be found when wrong type is stored")
		assert.Equal(t, logicalcluster.Name(""), clusterName, "Cluster name should be empty when wrong type is stored")
	})

	t.Run("WithClusterName_root_cluster", func(t *testing.T) {
		ctx := context.Background()
		rootCluster := logicalcluster.Name("root")

		ctxWithClusterName := ctxkeys.WithClusterName(ctx, rootCluster)
		retrievedClusterName, ok := ctxkeys.ClusterNameFromContext(ctxWithClusterName)
		assert.True(t, ok, "Root cluster name should be found in context")
		assert.Equal(t, rootCluster, retrievedClusterName, "Root cluster name should be retrievable")
	})

	t.Run("WithClusterName_empty_name", func(t *testing.T) {
		ctx := context.Background()
		emptyName := logicalcluster.Name("")

		ctxWithClusterName := ctxkeys.WithClusterName(ctx, emptyName)
		retrievedClusterName, ok := ctxkeys.ClusterNameFromContext(ctxWithClusterName)
		assert.True(t, ok, "Empty cluster name should still be found in context")
		assert.Equal(t, emptyName, retrievedClusterName, "Empty cluster name should be retrievable")
	})
}

func TestContextKeyIsolation(t *testing.T) {
	t.Run("different_keys_dont_interfere", func(t *testing.T) {
		ctx := context.Background()

		// Store values with different keys
		token := "test-token"
		workspace := "test-workspace"
		clusterName := logicalcluster.Name("test-cluster")

		ctx = ctxkeys.WithToken(ctx, token)
		ctx = ctxkeys.WithKcpWorkspace(ctx, workspace)
		ctx = ctxkeys.WithClusterName(ctx, clusterName)

		// Verify all values can be retrieved independently
		retrievedToken, tokenOk := ctxkeys.TokenFromContext(ctx)
		retrievedWorkspace, workspaceOk := ctxkeys.KcpWorkspaceFromContext(ctx)
		retrievedClusterName, clusterOk := ctxkeys.ClusterNameFromContext(ctx)

		assert.True(t, tokenOk, "Token should be retrievable")
		assert.True(t, workspaceOk, "Workspace should be retrievable")
		assert.True(t, clusterOk, "Cluster name should be retrievable")

		assert.Equal(t, token, retrievedToken, "Token should match")
		assert.Equal(t, workspace, retrievedWorkspace, "Workspace should match")
		assert.Equal(t, clusterName, retrievedClusterName, "Cluster name should match")
	})

	t.Run("overwriting_values", func(t *testing.T) {
		ctx := context.Background()

		// Store initial values
		initialToken := "initial-token"
		initialWorkspace := "initial-workspace"

		ctx = ctxkeys.WithToken(ctx, initialToken)
		ctx = ctxkeys.WithKcpWorkspace(ctx, initialWorkspace)

		// Overwrite with new values
		newToken := "new-token"
		newWorkspace := "new-workspace"

		ctx = ctxkeys.WithToken(ctx, newToken)
		ctx = ctxkeys.WithKcpWorkspace(ctx, newWorkspace)

		// Verify new values are retrieved
		retrievedToken, tokenOk := ctxkeys.TokenFromContext(ctx)
		retrievedWorkspace, workspaceOk := ctxkeys.KcpWorkspaceFromContext(ctx)

		assert.True(t, tokenOk, "Token should be retrievable")
		assert.True(t, workspaceOk, "Workspace should be retrievable")

		assert.Equal(t, newToken, retrievedToken, "Should get new token value")
		assert.Equal(t, newWorkspace, retrievedWorkspace, "Should get new workspace value")
	})
}
