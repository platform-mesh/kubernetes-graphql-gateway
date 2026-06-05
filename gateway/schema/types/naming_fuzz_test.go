package types_test

import (
	"regexp"
	"testing"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema/types"
)

// validIdentifier matches a valid GraphQL identifier: it must start with a
// letter or underscore and contain only letters, digits, and underscores.
var validIdentifier = regexp.MustCompile(`^[_a-zA-Z][_a-zA-Z0-9]*$`)

// FuzzSanitizeFieldName checks that SanitizeFieldName always returns a valid
// GraphQL identifier for any input and that the operation is idempotent.
func FuzzSanitizeFieldName(f *testing.F) {
	seeds := []string{
		"",
		"validFieldName",
		"field-name",
		"1field",
		"field.name-with$special",
		"_privateField",
		"!@#$%",
		"日本語",
		"a b\tc\n",
		"\x00\x01\x02",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, in string) {
		got := types.SanitizeFieldName(in)

		if !validIdentifier.MatchString(got) {
			t.Fatalf("SanitizeFieldName(%q) = %q, which is not a valid GraphQL identifier", in, got)
		}

		// Sanitizing an already-sanitized name must not change it.
		if again := types.SanitizeFieldName(got); again != got {
			t.Fatalf("SanitizeFieldName not idempotent: SanitizeFieldName(%q) = %q", got, again)
		}
	})
}
