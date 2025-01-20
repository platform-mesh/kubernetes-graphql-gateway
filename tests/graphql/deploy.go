package graphql

func CreateDeploymentMutation() string {
	return `mutation {
  apps {
    createDeployment(
      namespace: "default",
      object: {
        metadata: {
          name: "my-new-deployment"
          labels: {
            app: "my-app"
          }
        }
        spec: {
          replicas: 3
          selector: {
            matchLabels: {
              app: "my-app"
            }
          }
          template: {
            metadata: {
              labels: {
                app: "my-app"
              }
            }
            spec: {
              containers: [
                {
                  name: "nginx-container"
                  image: "nginx:latest"
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
        labels
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
      status {
        replicas
        availableReplicas
      }
    }
  }
}`
}

func SubscribeDeploymentsQuery() string {
	return `
		subscription {
			apps_deployments(namespace: "default") {
				metadata {
					name
				}
				spec {
					replicas
				}
			}
		}
	`
}
