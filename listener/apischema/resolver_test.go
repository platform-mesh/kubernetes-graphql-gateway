package apischema_test

import (
	"os"
	"path"
	"testing"

	"github.com/openmfp/crd-gql-gateway/listener/apischema"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	kubeconfigPath   = "../../.kcp/admin.kubeconfig"
	kubeconfigEnvVar = "KUBECONFIG"
	testDataDir      = "./testdata"
)

//TODO: refactor

func TestResolve(t *testing.T) {
	t.Skip()
	envErr := os.Setenv(kubeconfigEnvVar, kubeconfigPath)
	assert.NoError(t, envErr)
	kubeconfig := os.Getenv(kubeconfigEnvVar)
	assert.NotEmpty(t, kubeconfig)
	cfg, cErr := ctrl.GetConfig()
	assert.NoError(t, cErr)
	assert.NotNil(t, cfg)
	dc, dcErr := discovery.NewDiscoveryClientForConfig(cfg)
	assert.NoError(t, dcErr)
	assert.NotNil(t, dc)
	r := apischema.NewResolver()
	JSON, rErr := r.Resolve(dc)
	assert.NoError(t, rErr)
	assert.NotNil(t, JSON)
	wErr := os.WriteFile(path.Join(testDataDir, "kubeSchemaOut.json"), JSON, os.ModePerm)
	assert.NoError(t, wErr)
}
