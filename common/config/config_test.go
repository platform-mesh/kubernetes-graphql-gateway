package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_StructInitialization(t *testing.T) {
	cfg := Config{}

	// Test top-level fields
	assert.Empty(t, cfg.OpenApiDefinitionsPath)
	assert.False(t, cfg.EnableKcp)
	assert.False(t, cfg.LocalDevelopment)
	assert.False(t, cfg.IntrospectionAuthentication)

	// Test nested struct fields
	assert.Empty(t, cfg.Url.VirtualWorkspacePrefix)
	assert.Empty(t, cfg.Url.DefaultKcpWorkspace)
	assert.Empty(t, cfg.Url.GraphqlSuffix)

	assert.Empty(t, cfg.Listener.VirtualWorkspacesConfigPath)

	assert.Empty(t, cfg.Gateway.Port)
	assert.Empty(t, cfg.Gateway.UsernameClaim)
	assert.False(t, cfg.Gateway.ShouldImpersonate)

	assert.False(t, cfg.Gateway.HandlerCfg.Pretty)
	assert.False(t, cfg.Gateway.HandlerCfg.Playground)
	assert.False(t, cfg.Gateway.HandlerCfg.GraphiQL)

	assert.False(t, cfg.Gateway.Cors.Enabled)
	assert.Empty(t, cfg.Gateway.Cors.AllowedOrigins)
	assert.Empty(t, cfg.Gateway.Cors.AllowedHeaders)
}

func TestConfig_FieldAssignment(t *testing.T) {
	cfg := Config{
		OpenApiDefinitionsPath:      "/path/to/definitions",
		EnableKcp:                   true,
		LocalDevelopment:            true,
		IntrospectionAuthentication: true,
	}

	cfg.Url.VirtualWorkspacePrefix = "workspace"
	cfg.Url.DefaultKcpWorkspace = "default"
	cfg.Url.GraphqlSuffix = "graphql"

	cfg.Listener.VirtualWorkspacesConfigPath = "/path/to/config"

	cfg.Gateway.Port = "8080"
	cfg.Gateway.UsernameClaim = "email"
	cfg.Gateway.ShouldImpersonate = true

	cfg.Gateway.HandlerCfg.Pretty = true
	cfg.Gateway.HandlerCfg.Playground = true
	cfg.Gateway.HandlerCfg.GraphiQL = true

	cfg.Gateway.Cors.Enabled = true
	cfg.Gateway.Cors.AllowedOrigins = "*"
	cfg.Gateway.Cors.AllowedHeaders = "Authorization,Content-Type"

	// Verify assignments
	assert.Equal(t, "/path/to/definitions", cfg.OpenApiDefinitionsPath)
	assert.True(t, cfg.EnableKcp)
	assert.True(t, cfg.LocalDevelopment)
	assert.True(t, cfg.IntrospectionAuthentication)

	assert.Equal(t, "workspace", cfg.Url.VirtualWorkspacePrefix)
	assert.Equal(t, "default", cfg.Url.DefaultKcpWorkspace)
	assert.Equal(t, "graphql", cfg.Url.GraphqlSuffix)

	assert.Equal(t, "/path/to/config", cfg.Listener.VirtualWorkspacesConfigPath)

	assert.Equal(t, "8080", cfg.Gateway.Port)
	assert.Equal(t, "email", cfg.Gateway.UsernameClaim)
	assert.True(t, cfg.Gateway.ShouldImpersonate)

	assert.True(t, cfg.Gateway.HandlerCfg.Pretty)
	assert.True(t, cfg.Gateway.HandlerCfg.Playground)
	assert.True(t, cfg.Gateway.HandlerCfg.GraphiQL)

	assert.True(t, cfg.Gateway.Cors.Enabled)
	assert.Equal(t, "*", cfg.Gateway.Cors.AllowedOrigins)
	assert.Equal(t, "Authorization,Content-Type", cfg.Gateway.Cors.AllowedHeaders)
}

func TestConfig_NestedStructModification(t *testing.T) {
	cfg := Config{}

	// Test direct modification of nested structs
	cfg.Gateway.HandlerCfg = struct {
		Pretty     bool `mapstructure:"gateway-handler-pretty"`
		Playground bool `mapstructure:"gateway-handler-playground"`
		GraphiQL   bool `mapstructure:"gateway-handler-graphiql"`
	}{
		Pretty:     true,
		Playground: false,
		GraphiQL:   true,
	}

	assert.True(t, cfg.Gateway.HandlerCfg.Pretty)
	assert.False(t, cfg.Gateway.HandlerCfg.Playground)
	assert.True(t, cfg.Gateway.HandlerCfg.GraphiQL)
}

func TestConfig_MultipleInstances(t *testing.T) {
	cfg1 := Config{
		EnableKcp: true,
	}
	cfg1.Gateway.Port = "8080"

	cfg2 := Config{
		LocalDevelopment: true,
	}
	cfg2.Gateway.Port = "9090"

	// Verify independence
	assert.True(t, cfg1.EnableKcp)
	assert.False(t, cfg1.LocalDevelopment)
	assert.Equal(t, "8080", cfg1.Gateway.Port)

	assert.False(t, cfg2.EnableKcp)
	assert.True(t, cfg2.LocalDevelopment)
	assert.Equal(t, "9090", cfg2.Gateway.Port)
}
