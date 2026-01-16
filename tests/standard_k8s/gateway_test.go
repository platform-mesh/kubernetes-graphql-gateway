package standard_k8s_test

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestConfigMapCRUD tests the complete CRUD lifecycle for ConfigMaps via GraphQL
func (s *IntegrationTestSuite) TestConfigMapCRUD() {
	clusterName := s.uniqueName("configmap-test")
	ca := s.setupTestCluster(clusterName)
	defer s.cleanupTestCluster(ca)

	configMapName := s.uniqueName("test-cm")

	s.Run("List ConfigMaps", func() {
		result := s.executeGraphQL(clusterName, GraphQLRequest{
			Query: `
                query {
                    v1 {
                        ConfigMaps {
                            items {
                                metadata {
                                    name
                                    namespace
                                }
                            }
                        }
                    }
                }
            `,
		})

		s.Require().Empty(result.Errors)
		s.Require().NotNil(result.Data)
	})

	s.Run("Create ConfigMap", func() {
		result := s.executeGraphQL(clusterName, GraphQLRequest{
			Query: `
                mutation CreateConfigMap($namespace: String!, $object: ConfigMapInput!) {
                    v1 {
                        createConfigMap(namespace: $namespace, object: $object) {
                            metadata {
                                name
                                namespace
                            }
                            data
                        }
                    }
                }
            `,
			Variables: map[string]any{
				"namespace": "default",
				"object": map[string]any{
					"metadata": map[string]any{
						"name":      configMapName,
						"namespace": "default",
						"labels": map[string]string{
							"app":  "myapp",
							"tier": "backend",
						},
					},
					"data": map[string]string{
						"app":     "myapp",
						"version": "1.0",
					},
				},
			},
		})

		s.Require().Empty(result.Errors, "Create mutation failed: %+v", result.Errors)
		s.Require().NotNil(result.Data)

		// Verify the ConfigMap exists in Kubernetes
		cm := &corev1.ConfigMap{}
		err := s.k8sClient.Get(s.ctx, types.NamespacedName{
			Name:      configMapName,
			Namespace: "default",
		}, cm)
		s.Require().NoError(err, "ConfigMap should exist in cluster")
		s.Equal("myapp", cm.Data["app"])
		s.Equal("1.0", cm.Data["version"])
		s.Equal("myapp", cm.Labels["app"])
		s.Equal("backend", cm.Labels["tier"])
	})

	s.Run("Get ConfigMap", func() {
		result := s.executeGraphQL(clusterName, GraphQLRequest{
			Query: `
                query GetConfigMap($name: String!, $namespace: String!) {
                    v1 {
                        ConfigMap(name: $name, namespace: $namespace) {
                            metadata {
                                name
                                namespace
                                labels
                            }
                            data
                        }
                    }
                }
            `,
			Variables: map[string]any{
				"name":      configMapName,
				"namespace": "default",
			},
		})

		s.Require().Empty(result.Errors)
		s.Require().NotNil(result.Data)
	})

	s.Run("Query by label selector", func() {
		result := s.executeGraphQL(clusterName, GraphQLRequest{
			Query: `
                query ListConfigMaps($namespace: String!, $labelselector: String!) {
                    v1 {
                        ConfigMaps(namespace: $namespace, labelselector: $labelselector) {
                            items {
                                metadata {
                                    name
                                    labels
                                }
                            }
                        }
                    }
                }
            `,
			Variables: map[string]any{
				"namespace":     "default",
				"labelselector": "app=myapp",
			},
		})

		s.Require().Empty(result.Errors)
		s.Require().NotNil(result.Data)
	})

	s.Run("Update ConfigMap", func() {
		result := s.executeGraphQL(clusterName, GraphQLRequest{
			Query: `
                mutation UpdateConfigMap($name: String!, $namespace: String!, $object: ConfigMapInput!) {
                    v1 {
                        updateConfigMap(name: $name, namespace: $namespace, object: $object) {
                            metadata {
                                name
                                namespace
                            }
                            data
                        }
                    }
                }
            `,
			Variables: map[string]any{
				"name":      configMapName,
				"namespace": "default",
				"object": map[string]any{
					"metadata": map[string]any{
						"name":      configMapName,
						"namespace": "default",
						"labels": map[string]string{
							"app":  "myapp",
							"tier": "frontend",
						},
					},
					"data": map[string]string{
						"app":     "myapp",
						"version": "2.0",
						"env":     "production",
					},
				},
			},
		})

		s.Require().Empty(result.Errors, "Update mutation failed: %+v", result.Errors)
		s.Require().NotNil(result.Data)

		// Verify the update took effect in Kubernetes
		cm := &corev1.ConfigMap{}
		err := s.k8sClient.Get(s.ctx, types.NamespacedName{
			Name:      configMapName,
			Namespace: "default",
		}, cm)
		s.Require().NoError(err)
		s.Equal("2.0", cm.Data["version"], "Version should be updated")
		s.Equal("production", cm.Data["env"], "New field should be added")
		s.Equal("frontend", cm.Labels["tier"], "Label should be updated")
	})

	s.Run("Delete ConfigMap", func() {
		result := s.executeGraphQL(clusterName, GraphQLRequest{
			Query: `
                mutation DeleteConfigMap($name: String!, $namespace: String!) {
                    v1 {
                        deleteConfigMap(name: $name, namespace: $namespace)
                    }
                }
            `,
			Variables: map[string]any{
				"name":      configMapName,
				"namespace": "default",
			},
		})

		s.Require().Empty(result.Errors)

		// Verify the ConfigMap was deleted from Kubernetes
		s.Eventually(func() bool {
			cm := &corev1.ConfigMap{}
			err := s.k8sClient.Get(s.ctx, types.NamespacedName{
				Name:      configMapName,
				Namespace: "default",
			}, cm)
			return err != nil
		}, 5*time.Second, 100*time.Millisecond, "ConfigMap should be deleted")
	})
}
