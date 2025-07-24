package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstants(t *testing.T) {
	t.Run("categories_extension_key", func(t *testing.T) {
		assert.Equal(t, "x-kubernetes-categories", CategoriesExtensionKey)
		assert.NotEmpty(t, CategoriesExtensionKey)
	})

	t.Run("gvk_extension_key", func(t *testing.T) {
		assert.Equal(t, "x-kubernetes-group-version-kind", GVKExtensionKey)
		assert.NotEmpty(t, GVKExtensionKey)
	})

	t.Run("scope_extension_key", func(t *testing.T) {
		assert.Equal(t, "x-kubernetes-scope", ScopeExtensionKey)
		assert.NotEmpty(t, ScopeExtensionKey)
	})
}

func TestConstantsFormat(t *testing.T) {
	constants := []string{
		CategoriesExtensionKey,
		GVKExtensionKey,
		ScopeExtensionKey,
	}

	for _, constant := range constants {
		assert.True(t, strings.HasPrefix(constant, "x-kubernetes-"))
		assert.NotContains(t, constant, " ")
	}
}
