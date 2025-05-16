package schema_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	gatewaySchema "github.com/openmfp/kubernetes-graphql-gateway/gateway/schema"
)

func TestGateway_getNames(t *testing.T) {
	type testCase struct {
		name         string
		registry     map[string]string
		gvk          schema.GroupVersionKind
		wantSingular string
		wantPlural   string
	}

	tests := []testCase{
		{
			name:         "no_conflict",
			registry:     map[string]string{},
			gvk:          schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "Pod"},
			wantSingular: "Pod",
			wantPlural:   "Pods",
		},
		{
			name:         "same_kind_different_group_version",
			registry:     map[string]string{"Pod": "core/v1"},
			gvk:          schema.GroupVersionKind{Group: "custom.io", Version: "v2", Kind: "Pod"},
			wantSingular: "Pod_customio_v2",
			wantPlural:   "Pods_customio_v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gatewaySchema.GetGatewayForTest(tt.registry)
			gotSingular, gotPlural := g.GetNamesForTest(&tt.gvk)

			if gotSingular != tt.wantSingular || gotPlural != tt.wantPlural {
				t.Errorf("getNames() = (%q, %q), want (%q, %q)",
					gotSingular, gotPlural, tt.wantSingular, tt.wantPlural)
			}
		})
	}
}
