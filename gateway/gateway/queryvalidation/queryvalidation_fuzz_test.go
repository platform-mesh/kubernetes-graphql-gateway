package queryvalidation

import "testing"

func FuzzQueryValidation(f *testing.F) {
	f.Add("{ users { name } }")
	f.Add("query { deeply { nested { query { that { goes { deep } } } } } }")
	f.Add("")
	f.Add("{")
	f.Add("mutation { createPod { metadata { name } } }")
	f.Add("query { ...F } fragment F on Query { a { b } }")

	f.Fuzz(func(t *testing.T, query string) {
		cfg := Config{MaxDepth: 10, MaxComplexity: 100}
		_ = Validate(query, cfg)
	})
}
