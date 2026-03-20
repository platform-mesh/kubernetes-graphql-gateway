package types_test

import (
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"
	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSanitizeGroupName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "simple group", input: "apps", want: "apps"},
		{name: "group with dots", input: "networking.k8s.io", want: "networking_k8s_io"},
		{name: "group with hyphens", input: "my-group", want: "my_group"},
		{name: "group starting with number", input: "1group", want: "_1group"},
		{name: "group with special chars", input: "my@group!", want: "my_group_"},
		{name: "already valid", input: "_valid_group", want: "_valid_group"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, types.SanitizeGroupName(tt.input))
		})
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := types.NewRegistry()

	// Create test types
	outputType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "TestType",
		Fields: graphql.Fields{"name": &graphql.Field{Type: graphql.String}},
	})
	inputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   "TestTypeInput",
		Fields: graphql.InputObjectConfigFieldMap{"name": &graphql.InputObjectFieldConfig{Type: graphql.String}},
	})

	// Register types
	registry.Register("test-key", outputType, inputType)

	// Get types back
	gotOutput, gotInput := registry.Get("test-key")
	assert.Equal(t, outputType, gotOutput)
	assert.Equal(t, inputType, gotInput)
}

func TestRegistry_GetNonExistent(t *testing.T) {
	registry := types.NewRegistry()

	output, input := registry.Get("non-existent")
	assert.Nil(t, output)
	assert.Nil(t, input)
}

func TestRegistry_ProcessingState(t *testing.T) {
	registry := types.NewRegistry()
	key := "test-key"

	// Initially not processing
	assert.False(t, registry.IsProcessing(key))

	// Mark as processing
	registry.MarkProcessing(key)
	assert.True(t, registry.IsProcessing(key))

	// Get should return nil while processing
	output, input := registry.Get(key)
	assert.Nil(t, output)
	assert.Nil(t, input)

	// Unmark processing
	registry.UnmarkProcessing(key)
	assert.False(t, registry.IsProcessing(key))
}

func TestRegistry_GetUniqueTypeName(t *testing.T) {
	registry := types.NewRegistry()

	tests := []struct {
		name     string
		gvk      schema.GroupVersionKind
		expected string
	}{
		{
			name:     "first registration",
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: "Deployment",
		},
		{
			name:     "same kind same group",
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: "Deployment",
		},
		{
			name:     "same kind different group",
			gvk:      schema.GroupVersionKind{Group: "extensions", Version: "v1beta1", Kind: "Deployment"},
			expected: "ExtensionsV1beta1Deployment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.GetUniqueTypeName(&tt.gvk)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRegistry_GetUniqueTypeName_EmptyGroup(t *testing.T) {
	registry := types.NewRegistry()

	// First: register Pod with empty group (core API)
	gvk1 := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	assert.Equal(t, "Pod", registry.GetUniqueTypeName(&gvk1))

	// Second: same kind, same group should return same name
	assert.Equal(t, "Pod", registry.GetUniqueTypeName(&gvk1))
}

func TestRegistry_GetUniqueTypeName_GroupWithDots(t *testing.T) {
	registry := types.NewRegistry()

	// First: register Ingress with networking.k8s.io group
	gvk1 := schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}
	assert.Equal(t, "Ingress", registry.GetUniqueTypeName(&gvk1))

	// Second: same kind from extensions group should get prefixed with sanitized group
	gvk2 := schema.GroupVersionKind{Group: "extensions", Version: "v1beta1", Kind: "Ingress"}
	assert.Equal(t, "ExtensionsV1beta1Ingress", registry.GetUniqueTypeName(&gvk2))
}

func TestRegistry_IsProcessing_AfterMark(t *testing.T) {
	registry := types.NewRegistry()

	assert.False(t, registry.IsProcessing("test-key"))

	registry.MarkProcessing("test-key")
	assert.True(t, registry.IsProcessing("test-key"))
}
