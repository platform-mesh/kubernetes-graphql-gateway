package types

import (
	"regexp"
	"strings"
)

var (
	// invalidFieldCharRegex matches characters that are not valid in GraphQL field names
	invalidFieldCharRegex = regexp.MustCompile(`[^_a-zA-Z0-9]`)
	// validFieldStartRegex matches valid starting characters for GraphQL field names
	validFieldStartRegex = regexp.MustCompile(`^[_a-zA-Z]`)
)

// SanitizeFieldName converts a field name to a valid GraphQL identifier.
// It replaces invalid characters with underscores and prepends '_' if needed.
func SanitizeFieldName(name string) string {
	// Replace any invalid characters with '_'
	name = invalidFieldCharRegex.ReplaceAllString(name, "_")

	// If the name doesn't start with a letter or underscore, prepend '_'
	if !validFieldStartRegex.MatchString(name) {
		name = "_" + name
	}

	return name
}

// GenerateTypeName creates a type name from a prefix and field path.
// This is used to generate unique names for nested types.
func GenerateTypeName(typePrefix string, fieldPath []string) string {
	return typePrefix + strings.Join(fieldPath, "")
}
