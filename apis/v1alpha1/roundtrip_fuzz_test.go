package v1alpha1

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
)

func FuzzClusterAccessRoundTrip(f *testing.F) {
	f.Add([]byte(`{"metadata":{"name":"test"},"spec":{"host":"https://example.com"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"spec":{"auth":{"tokenSecretRef":{"name":"s","namespace":"ns","key":"token"}}}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzRoundTrip(t, data, &ClusterAccess{}, &ClusterAccess{})
	})
}

func fuzzRoundTrip[T any](t *testing.T, data []byte, obj *T, obj2 *T) {
	t.Helper()

	if err := json.Unmarshal(data, obj); err != nil {
		return
	}

	roundtripped, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if err := json.Unmarshal(roundtripped, obj2); err != nil {
		t.Fatalf("failed to unmarshal roundtripped data: %v", err)
	}

	if !equality.Semantic.DeepEqual(obj, obj2) {
		t.Errorf("roundtrip mismatch for %T", obj)
	}
}
