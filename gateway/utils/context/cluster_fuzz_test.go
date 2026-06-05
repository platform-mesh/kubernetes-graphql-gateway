package context

import (
	"testing"
)

// FuzzFindClusterTarget verifies that any value FindClusterTarget returns is
// safe to use as a cluster routing target: it is either empty, or it satisfies
// the validClusterTarget regex and the maximum length bound. This guards the
// security-critical invariant that untrusted "clusterTarget" extension values
// can never slip through unvalidated.
func FuzzFindClusterTarget(f *testing.F) {
	seeds := []string{
		"",
		"my-cluster",
		"root:org:workspace",
		"cluster_1",
		"-leading-dash",
		"has space",
		"emoji😀",
		"../traversal",
		"with\x00null",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, target string) {
		reqs := []GraphQLRequest{{
			Extensions: map[string]any{"clusterTarget": target},
		}}

		got := FindClusterTarget(reqs)

		if got == "" {
			return // empty is always safe
		}
		if len(got) > maxClusterTargetLen {
			t.Fatalf("FindClusterTarget returned %q exceeding max length %d", got, maxClusterTargetLen)
		}
		if !validClusterTarget.MatchString(got) {
			t.Fatalf("FindClusterTarget returned %q which does not match validClusterTarget", got)
		}
		// A non-empty result must be exactly the input we provided.
		if got != target {
			t.Fatalf("FindClusterTarget returned %q, want %q (the only candidate)", got, target)
		}
	})
}
