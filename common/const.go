package common

import "time"

const (
	CategoriesExtensionKey = "x-kubernetes-categories"
	GVKExtensionKey        = "x-kubernetes-group-version-kind"
	ScopeExtensionKey      = "x-kubernetes-scope"

	// Timeout constants for different test scenarios
	ShortTimeout = 100 * time.Millisecond // Short timeout for quick operations
	LongTimeout  = 2 * time.Second        // Longer timeout for file system operations
)
