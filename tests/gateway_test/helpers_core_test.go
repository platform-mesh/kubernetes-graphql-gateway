package gateway_test

type podData struct {
	Metadata metadata `json:"metadata"`
	Spec     podSpec  `json:"spec"`
}

type podSpec struct {
	Containers []container `json:"containers"`
}

type container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func createPodMutation() string {
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

func getPodQuery() string {
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

func deletePodMutation() string {
	return `
    mutation {
      core {
        deletePod(name: "test-pod", namespace: "default")
      }
    }
    `
}
