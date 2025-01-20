package graphql

type PodData struct {
	Metadata Metadata `json:"metadata"`
	Spec     PodSpec  `json:"spec"`
}

type PodSpec struct {
	Containers []Container `json:"containers"`
}

type Container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func CreatePodMutation() string {
	return `
    mutation {
      core {
        createPod(
          namespace: "default",
          object: {
            metadata: { name: "test-pod" },
            spec: {
              containers: [
                {
                  name: "test-container",
                  image: "nginx"
                }
              ]
            }
          }
        ) {
          metadata {
            name
            namespace
          }
          spec {
            containers {
              name
              image
            }
          }
        }
      }
    }
    `
}

func GetPodQuery() string {
	return `
    query {
      core {
        Pod(name: "test-pod", namespace: "default") {
          metadata {
            name
            namespace
          }
          spec {
            containers {
              name
              image
            }
          }
        }
      }
    }
    `
}

func DeletePodMutation() string {
	return `
    mutation {
      core {
        deletePod(name: "test-pod", namespace: "default")
      }
    }
    `
}
