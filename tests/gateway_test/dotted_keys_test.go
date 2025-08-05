package gateway_test

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/stretchr/testify/require"
)

// TestDottedKeysIntegration tests all dotted key fields in a single Deployment resource using stringMapInput scalar
func (suite *CommonTestSuite) TestDottedKeysIntegration() {
	workspaceName := "dottedKeysWorkspace"

	require.NoError(suite.T(), suite.writeToFileWithClusterMetadata(
		filepath.Join("testdata", "kubernetes"),
		filepath.Join(suite.appCfg.OpenApiDefinitionsPath, workspaceName),
	))

	url := fmt.Sprintf("%s/%s/graphql", suite.server.URL, workspaceName)

	// Create the Deployment with all dotted key fields using variables
	createResp, statusCode, err := suite.sendAuthenticatedRequestWithVariables(url, createDeploymentWithDottedKeys(), getDeploymentVariables())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), createResp.Errors, "GraphQL errors: %v", createResp.Errors)

	// Get the Deployment and verify all dotted key fields
	getResp, statusCode, err := suite.sendAuthenticatedRequest(url, getDeploymentWithDottedKeys())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), getResp.Errors, "GraphQL errors: %v", getResp.Errors)

	deployment := getResp.Data.Apps.Deployment
	require.Equal(suite.T(), "dotted-keys-deployment", deployment.Metadata.Name)
	require.Equal(suite.T(), "default", deployment.Metadata.Namespace)

	// Verify metadata.labels with dotted keys (direct map)
	labels := deployment.Metadata.Labels
	require.NotNil(suite.T(), labels)
	labelsMap, ok := labels.(map[string]interface{})
	require.True(suite.T(), ok, "Expected labels to be a map")
	require.Len(suite.T(), labelsMap, 3)
	require.Equal(suite.T(), "my-app", labelsMap["app.kubernetes.io/name"])
	require.Equal(suite.T(), "1.0.0", labelsMap["app.kubernetes.io/version"])
	require.Equal(suite.T(), "production", labelsMap["environment"])

	// Verify metadata.annotations with dotted keys (direct map)
	annotations := deployment.Metadata.Annotations
	require.NotNil(suite.T(), annotations)
	annotationsMap, ok := annotations.(map[string]interface{})
	require.True(suite.T(), ok, "Expected annotations to be a map")
	require.Len(suite.T(), annotationsMap, 2)
	require.Equal(suite.T(), "1", annotationsMap["deployment.kubernetes.io/revision"])
	require.Contains(suite.T(), annotationsMap["kubectl.kubernetes.io/last-applied-configuration"], "apiVersion")

	// Verify spec.selector.matchLabels with dotted keys (direct map)
	matchLabels := deployment.Spec.Selector.MatchLabels
	require.NotNil(suite.T(), matchLabels)
	matchLabelsMap, ok := matchLabels.(map[string]interface{})
	require.True(suite.T(), ok, "Expected matchLabels to be a map")
	require.Len(suite.T(), matchLabelsMap, 2)
	require.Equal(suite.T(), "my-app", matchLabelsMap["app.kubernetes.io/name"])
	require.Equal(suite.T(), "frontend", matchLabelsMap["app.kubernetes.io/component"])

	// Verify spec.template.spec.nodeSelector with dotted keys (direct map)
	nodeSelector := deployment.Spec.Template.Spec.NodeSelector
	require.NotNil(suite.T(), nodeSelector)
	nodeSelectorMap, ok := nodeSelector.(map[string]interface{})
	require.True(suite.T(), ok, "Expected nodeSelector to be a map")
	require.Len(suite.T(), nodeSelectorMap, 2)
	require.Equal(suite.T(), "amd64", nodeSelectorMap["kubernetes.io/arch"])
	require.Equal(suite.T(), "m5.large", nodeSelectorMap["node.kubernetes.io/instance-type"])

	// Clean up: Delete the Deployment
	deleteResp, statusCode, err := suite.sendAuthenticatedRequest(url, deleteDeploymentMutation())
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), http.StatusOK, statusCode, "Expected status code 200")
	require.Nil(suite.T(), deleteResp.Errors, "GraphQL errors: %v", deleteResp.Errors)
}

func createDeploymentWithDottedKeys() string {
	return `
	mutation createDeploymentWithDottedKeys(
		$labels: StringMapInput,
		$annotations: StringMapInput,
		$matchLabels: StringMapInput,
		$templateLabels: StringMapInput,
		$nodeSelector: StringMapInput
	) {
		apps {
			createDeployment(
				namespace: "default"
				object: {
					metadata: {
						name: "dotted-keys-deployment"
						labels: $labels
						annotations: $annotations
					}
					spec: {
						replicas: 2
						selector: {
							matchLabels: $matchLabels
						}
						template: {
							metadata: {
								labels: $templateLabels
							}
							spec: {
								nodeSelector: $nodeSelector
								containers: [
									{
										name: "web"
										image: "nginx:1.21"
										ports: [
											{
												containerPort: 80
											}
										]
									}
								]
							}
						}
					}
				}
			) {
				metadata {
					name
					namespace
				}
			}
		}
	}
	`
}

func getDeploymentVariables() map[string]interface{} {
	return map[string]interface{}{
		"labels": []map[string]string{
			{"key": "app.kubernetes.io/name", "value": "my-app"},
			{"key": "app.kubernetes.io/version", "value": "1.0.0"},
			{"key": "environment", "value": "production"},
		},
		"annotations": []map[string]string{
			{"key": "deployment.kubernetes.io/revision", "value": "1"},
			{"key": "kubectl.kubernetes.io/last-applied-configuration", "value": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\"}"},
		},
		"matchLabels": []map[string]string{
			{"key": "app.kubernetes.io/name", "value": "my-app"},
			{"key": "app.kubernetes.io/component", "value": "frontend"},
		},
		"templateLabels": []map[string]string{
			{"key": "app.kubernetes.io/name", "value": "my-app"},
			{"key": "app.kubernetes.io/component", "value": "frontend"},
		},
		"nodeSelector": []map[string]string{
			{"key": "kubernetes.io/arch", "value": "amd64"},
			{"key": "node.kubernetes.io/instance-type", "value": "m5.large"},
		},
	}
}

func getDeploymentWithDottedKeys() string {
	return `
	query {
		apps {
			Deployment(namespace: "default", name: "dotted-keys-deployment") {
				metadata {
					name
					namespace
					labels
					annotations
				}
				spec {
					replicas
					selector {
						matchLabels
					}
					template {
						metadata {
							labels
						}
						spec {
							nodeSelector
							containers {
								name
								image
								ports {
									containerPort
								}
							}
						}
					}
				}
			}
		}
	}
	`
}

func deleteDeploymentMutation() string {
	return `
	mutation {
		apps {
			deleteDeployment(namespace: "default", name: "dotted-keys-deployment")
		}
	}
	`
}
