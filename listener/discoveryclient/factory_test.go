package discoveryclient

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewFactory(t *testing.T) {
	tests := map[string]struct {
		inputCfg  *rest.Config
		expectErr bool
	}{
		"valid_config": {inputCfg: &rest.Config{}, expectErr: false},
		"nil_config":   {inputCfg: nil, expectErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			factory, err := NewFactory(tc.inputCfg)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, factory)
			assert.Equal(t, factory.Config, tc.inputCfg)
		})
	}
}

func TestClientForCluster(t *testing.T) {
	tests := map[string]struct {
		clusterName string
		restCfg     *rest.Config
		expectErr   bool
	}{
		"invalid_config": {clusterName: "test-cluster", restCfg: &rest.Config{Host: "://192.168.1.13:6443"}, expectErr: true},
		"valid_config":   {clusterName: "test-cluster", restCfg: &rest.Config{Host: "https://192.168.1.13:6443"}, expectErr: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			factory := &FactoryProvider{
				Config:             tc.restCfg,
				NewDiscoveryIFFunc: fakeClientFactory,
			}
			dc, err := factory.ClientForCluster(tc.clusterName)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, dc)
		})
	}
}

func TestRestMapperForCluster(t *testing.T) {
	tests := map[string]struct {
		clusterName string
		restCfg     *rest.Config
		expectErr   bool
	}{
		"invalid_config": {clusterName: "test-cluster", restCfg: &rest.Config{Host: "://192.168.1.13:6443"}, expectErr: true},
		"valid_config":   {clusterName: "test-cluster", restCfg: &rest.Config{Host: "https://192.168.1.13:6443"}, expectErr: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			factory := &FactoryProvider{
				Config:             tc.restCfg,
				NewDiscoveryIFFunc: fakeClientFactory,
			}
			rm, err := factory.RestMapperForCluster(tc.clusterName)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, rm)
		})
	}
}

func fakeClientFactory(_ *rest.Config) (discovery.DiscoveryInterface, error) {
	client := fakeclientset.NewClientset()
	fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		return nil, errors.New("failed to get fake discovery client")
	}
	return fakeDiscovery, nil
}
