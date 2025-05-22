package apischema

import "testing"

// Compile-time check that ResolverProvider implements Resolver interface
var _ Resolver = (*ResolverProvider)(nil)

// TestNewResolverNotNil checks if NewResolver() returns a non-nil *ResolverProvider
// instance. This is a runtime check to ensure that the function behaves as expected.
func TestNewResolverNotNil(t *testing.T) {
	r := NewResolver()
	if r == nil {
		t.Fatal("NewResolver() returned nil, expected non-nil *ResolverProvider")
	}
}
