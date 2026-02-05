package types_test

import (
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := types.NewRegistry(func(s string) string { return s })

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
	if gotOutput != outputType {
		t.Errorf("Get() output = %v, want %v", gotOutput, outputType)
	}
	if gotInput != inputType {
		t.Errorf("Get() input = %v, want %v", gotInput, inputType)
	}

}

func TestRegistry_GetNonExistent(t *testing.T) {
	registry := types.NewRegistry(func(s string) string { return s })

	output, input := registry.Get("non-existent")
	if output != nil || input != nil {
		t.Errorf("Get() for non-existent key should return nil, got output=%v, input=%v", output, input)
	}
}

func TestRegistry_ProcessingState(t *testing.T) {
	registry := types.NewRegistry(func(s string) string { return s })

	key := "test-key"

	// Initially not processing
	if registry.IsProcessing(key) {
		t.Error("IsProcessing() should return false for new key")
	}

	// Mark as processing
	registry.MarkProcessing(key)
	if !registry.IsProcessing(key) {
		t.Error("IsProcessing() should return true after MarkProcessing()")
	}

	// Get should return nil while processing
	output, input := registry.Get(key)
	if output != nil || input != nil {
		t.Error("Get() should return nil for processing type")
	}

	// Unmark processing
	registry.UnmarkProcessing(key)
	if registry.IsProcessing(key) {
		t.Error("IsProcessing() should return false after UnmarkProcessing()")
	}
}

func TestRegistry_GetUniqueTypeName(t *testing.T) {
	sanitize := func(s string) string {
		return s + "_sanitized"
	}
	registry := types.NewRegistry(sanitize)

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
			expected: "ExtensionsSanitizedV1beta1Deployment", // flect.Pascalize removes underscores
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := registry.GetUniqueTypeName(&tt.gvk)
			if got != tt.expected {
				t.Errorf("GetUniqueTypeName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRegistry_GetUniqueTypeName_EmptyGroup(t *testing.T) {
	registry := types.NewRegistry(func(s string) string { return s })

	// First: register Pod with empty group (core API)
	gvk1 := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	got1 := registry.GetUniqueTypeName(&gvk1)
	if got1 != "Pod" {
		t.Errorf("GetUniqueTypeName() for core API = %q, want %q", got1, "Pod")
	}

	// Second: same kind, same group should return same name
	got2 := registry.GetUniqueTypeName(&gvk1)
	if got2 != "Pod" {
		t.Errorf("GetUniqueTypeName() for same GVK = %q, want %q", got2, "Pod")
	}
}

func TestRegistry_IsProcessing_AfterMark(t *testing.T) {
	registry := types.NewRegistry(func(s string) string { return s })

	if registry.IsProcessing("test-key") {
		t.Error("IsProcessing() should return false for non-existent key")
	}

	registry.MarkProcessing("test-key")
	if !registry.IsProcessing("test-key") {
		t.Error("IsProcessing() should return true after MarkProcessing()")
	}
}
