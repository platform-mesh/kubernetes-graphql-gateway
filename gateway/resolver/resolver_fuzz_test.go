package resolver

import "testing"

func FuzzParseAndValidateYAML(f *testing.F) {
	f.Add("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test")
	f.Add("")
	f.Add("invalid: [yaml")
	f.Add("---\napiVersion: v1\nkind: Pod\nmetadata:\n  name: test\n  namespace: default")
	f.Add("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: first\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: second")

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parseAndValidateYAML(input)
	})
}
