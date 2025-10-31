package standard_k8s_test

import (
	"os"
	"path/filepath"
	"time"

	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
)

func (s *IntegrationTestSuite) TestClusterAccessLifecycle() {
	name := s.uniqueName("test-cluster")
	ca := s.createClusterAccessForEnvtest(name)

	s.Run("create", func() {
		err := s.k8sClient.Create(s.ctx, ca)
		s.Require().NoError(err)

		hasCA := ca.Spec.CA != nil && ca.Spec.CA.SecretRef != nil
		hasAuth := ca.Spec.Auth != nil
		testLog.Info("Created ClusterAccess",
			"name", name,
			"host", ca.Spec.Host,
			"hasCA", hasCA,
			"hasAuth", hasAuth)
	})

	s.Run("get", func() {
		retrieved := &gatewayv1alpha1.ClusterAccess{}
		err := s.k8sClient.Get(s.ctx, types.NamespacedName{Name: name}, retrieved)
		s.Require().NoError(err)
		s.Equal(name, retrieved.Name)
		s.Equal(testCfg.Host, retrieved.Spec.Host)

		testLog.Info("Retrieved ClusterAccess", "name", name)
	})

	s.Run("wait_for_schema_file", func() {
		var schemaContent []byte
		s.Eventually(func() bool {
			var err error
			schemaContent, err = os.ReadFile(filepath.Join(s.schemaDir, name))
			return err == nil && len(schemaContent) > 0
		}, 10*time.Second, 500*time.Millisecond, "Schema file should be created by listener")

		testLog.Info("Schema file created",
			"name", name,
			"schemaSize", len(schemaContent))
	})

	s.Run("delete", func() {
		err := s.k8sClient.Delete(s.ctx, ca)
		s.Require().NoError(err)

		testLog.Info("Deleted ClusterAccess", "name", name)
		// TODO: Verify that the schema file is deleted upon ClusterAccess deletion
	})
}
