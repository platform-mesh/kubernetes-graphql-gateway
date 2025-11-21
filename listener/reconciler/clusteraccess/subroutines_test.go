package clusteraccess

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/platform-mesh/golang-commons/logger"
	gatewayv1alpha1 "github.com/platform-mesh/kubernetes-graphql-gateway/common/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/pkg/workspacefile"
	"github.com/platform-mesh/kubernetes-graphql-gateway/listener/reconciler"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGenerateSchemaSubroutine_Process_InvalidResourceType(t *testing.T) {
	tmp := t.TempDir()
	fh, err := workspacefile.NewIOHandler(tmp)
	assert.NoError(t, err)
	log, _ := logger.New(logger.DefaultConfig())

	r := &ClusterAccessReconciler{
		ioHandler: fh,
		log:       log,
	}
	s := &generateSchemaSubroutine{reconciler: r}

	_, opErr := s.Process(context.Background(), &metav1.PartialObjectMetadata{})

	assert.NotNil(t, opErr)
}

func TestGenerateSchemaSubroutine_Process_MissingHostReturnsError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = gatewayv1alpha1.AddToScheme(scheme)

	ca := &gatewayv1alpha1.ClusterAccess{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-cluster",
			Annotations: map[string]string{},
		},
		Spec: gatewayv1alpha1.ClusterAccessSpec{
			// Host is intentionally empty to trigger validation error
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ca).Build()

	tmp := t.TempDir()
	fh, err := workspacefile.NewIOHandler(tmp)
	assert.NoError(t, err)

	log, _ := logger.New(logger.DefaultConfig())

	r := &ClusterAccessReconciler{
		ioHandler: fh,
		log:       log,
		opts: reconciler.ReconcilerOpts{
			Client:      fakeClient,
			Config:      &rest.Config{Host: "https://unit-test.invalid"},
			ManagerOpts: ctrl.Options{Scheme: scheme},
			Scheme:      scheme,
		},
	}
	s := &generateSchemaSubroutine{reconciler: r}

	res, opErr := s.Process(context.Background(), ca)

	assert.NotNil(t, opErr)
	assert.Equal(t, ctrl.Result{}, res)
}

func TestGenerateSchemaSubroutine_Finalize_DeletesCurrentAndPreviousPaths(t *testing.T) {
	tmp := t.TempDir()
	fh, err := workspacefile.NewIOHandler(tmp)
	assert.NoError(t, err)
	log, _ := logger.New(logger.DefaultConfig())

	// Create both files that should be deleted
	err = os.MkdirAll(filepath.Join(tmp, filepath.Dir("current-path")), 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmp, "current-path"), []byte("data"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmp, "previous-path"), []byte("data"), 0o644)
	assert.NoError(t, err)

	r := &ClusterAccessReconciler{
		ioHandler: fh,
		log:       log,
	}
	s := &generateSchemaSubroutine{reconciler: r}

	ca := &gatewayv1alpha1.ClusterAccess{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-resource",
		},
		Spec: gatewayv1alpha1.ClusterAccessSpec{
			Path: "current-path",
		},
		Status: gatewayv1alpha1.ClusterAccessStatus{
			ObservedPath: "previous-path",
		},
	}

	res, opErr := s.Finalize(context.Background(), ca)

	assert.Nil(t, opErr)
	assert.Equal(t, ctrl.Result{}, res)
	// Verify files are deleted
	_, err = os.Stat(filepath.Join(tmp, "current-path"))
	assert.Error(t, err)
	_, err = os.Stat(filepath.Join(tmp, "previous-path"))
	assert.Error(t, err)
}

func TestGenerateSchemaSubroutine_restMapperFromConfig_SucceedsWithMinimalConfig(t *testing.T) {
	tmp := t.TempDir()
	fh, err := workspacefile.NewIOHandler(tmp)
	assert.NoError(t, err)
	log, _ := logger.New(logger.DefaultConfig())

	r := &ClusterAccessReconciler{
		ioHandler: fh,
		log:       log,
	}
	s := &generateSchemaSubroutine{reconciler: r}

	rm, err := s.restMapperFromConfig(&rest.Config{})

	assert.NotNil(t, rm)
	assert.NoError(t, err)
}
