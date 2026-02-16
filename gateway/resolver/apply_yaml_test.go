// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and platform-mesh contributors
// SPDX-License-Identifier: Apache-2.0

package resolver_test

import (
	"context"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/golang-commons/logger/testlogger"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyYaml(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	log := testlogger.New().Logger

	tests := []struct {
		name           string
		existingObjs   []client.Object
		args           map[string]any
		wantErr        bool
		errContains    string
		validateResult func(t *testing.T, result any, fakeClient client.Client)
	}{
		{
			name:         "create new ConfigMap",
			existingObjs: nil,
			args: map[string]any{
				"yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value`,
			},
			wantErr: false,
			validateResult: func(t *testing.T, result any, fakeClient client.Client) {
				resultMap, ok := result.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test-config", resultMap["metadata"].(map[string]any)["name"])

				var cm corev1.ConfigMap
				err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-config", Namespace: "default"}, &cm)
				require.NoError(t, err)
				assert.Equal(t, "value", cm.Data["key"])
			},
		},
		{
			name: "update existing ConfigMap",
			existingObjs: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-config",
						Namespace:       "default",
						ResourceVersion: "123",
					},
					Data: map[string]string{"key": "old-value"},
				},
			},
			args: map[string]any{
				"yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: new-value`,
			},
			wantErr: false,
			validateResult: func(t *testing.T, result any, fakeClient client.Client) {
				resultMap, ok := result.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test-config", resultMap["metadata"].(map[string]any)["name"])

				var cm corev1.ConfigMap
				err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-config", Namespace: "default"}, &cm)
				require.NoError(t, err)
				assert.Equal(t, "new-value", cm.Data["key"])
			},
		},
		{
			name:         "invalid YAML syntax",
			existingObjs: nil,
			args: map[string]any{
				"yaml": `invalid: yaml: content: [`,
			},
			wantErr:     true,
			errContains: "invalid YAML",
		},
		{
			name:         "missing kind field",
			existingObjs: nil,
			args: map[string]any{
				"yaml": `apiVersion: v1
metadata:
  name: test-config`,
			},
			wantErr:     true,
			errContains: "YAML must contain kind field",
		},
		{
			name:         "missing yaml argument",
			existingObjs: nil,
			args:         map[string]any{},
			wantErr:      true,
			errContains:  "missing required argument",
		},
		{
			name:         "dry run does not persist",
			existingObjs: nil,
			args: map[string]any{
				"yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: dry-run-config
  namespace: default
data:
  key: value`,
				"dryRun": true,
			},
			wantErr: false,
			validateResult: func(t *testing.T, result any, fakeClient client.Client) {
				resultMap, ok := result.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "dry-run-config", resultMap["metadata"].(map[string]any)["name"])

				var cm corev1.ConfigMap
				err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "dry-run-config", Namespace: "default"}, &cm)
				require.Error(t, err)
			},
		},
		{
			name:         "cluster-scoped resource (Namespace)",
			existingObjs: nil,
			args: map[string]any{
				"yaml": `apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace`,
			},
			wantErr: false,
			validateResult: func(t *testing.T, result any, fakeClient client.Client) {
				resultMap, ok := result.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test-namespace", resultMap["metadata"].(map[string]any)["name"])

				var ns corev1.Namespace
				err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-namespace"}, &ns)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.existingObjs...).
				Build()

			svc := resolver.New(log, fakeClient)

			resolverFn := svc.ApplyYaml()

			result, err := resolverFn(graphql.ResolveParams{
				Context: context.Background(),
				Args:    tt.args,
			})

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			if tt.validateResult != nil {
				tt.validateResult(t, result, fakeClient)
			}
		})
	}
}
