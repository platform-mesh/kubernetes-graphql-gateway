package gateway

import (
	"context"
	"github.com/graphql-go/graphql"
	"github.com/stretchr/testify/require"
	"log"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (suite *CommonTestSuite) TestSchemaSubscribe() {
	ctx, cancel := context.WithCancel(context.Background())
	c := graphql.Subscribe(graphql.Params{
		Context:       ctx,
		RequestString: SubscribeDeployments(),
		Schema:        suite.schema,
	})

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for res := range c {
			data, ok := res.Data.(map[string]interface{})
			if !ok {
				log.Fatalf("Error asserting res.Data to map")
			}

			require.Equal(suite.T(), "my-new-deployment", data["apps_deployments"].([]interface{})[0].(map[string]interface{})["metadata"].(map[string]interface{})["name"])
			wg.Done()
		}
	}()

	err := suite.runtimeClient.Create(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-new-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "my-app"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	})

	require.NoError(suite.T(), err)

	wg.Wait()
	cancel()
}

func SubscribeDeployments() string {
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
