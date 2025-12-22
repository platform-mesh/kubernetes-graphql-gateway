package resolver_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/platform-mesh/golang-commons/logger/testlogger"
	commonmocks "github.com/platform-mesh/kubernetes-graphql-gateway/common/mocks"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

func TestSubscribeItems_NoResourceVersion_EmitsInitialAddedAndWatchesFromListRV(t *testing.T) {
	log := testlogger.New().Logger

	mockClient := &commonmocks.MockWithWatch{}
	defer mockClient.AssertExpectations(t)

	gvk := schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "ConfigMap"}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			lst := args[1].(*unstructured.UnstructuredList)
			lst.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
			lst.SetResourceVersion("rv-list-123")
			lst.Items = []unstructured.Unstructured{
				*makeObject("default", "cm-a", "10", gvk),
				*makeObject("default", "cm-b", "11", gvk),
			}
		}).
		Return(nil).
		Once()

	fakeWatcher := watch.NewFake()
	mockClient.On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(fakeWatcher, nil).
		Once()

	svc := resolver.New(log, mockClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := map[string]any{
		resolver.SubscribeToAllArg: true,
	}

	p := graphql.ResolveParams{Context: ctx, Args: args}

	field := svc.SubscribeItems(gvk, apiextensionsv1.ClusterScoped)
	chAny, err := field(p)
	require.NoError(t, err)
	ch := chAny.(chan any)

	for _, name := range []string{"cm-a", "cm-b"} {
		evt := receiveEvent[resolver.SubscriptionEnvelope](t, ctx, ch)
		require.Equal(t, resolver.EventTypeAdded, evt.Type)
		obj := evt.Object.(map[string]any)
		md := obj["metadata"].(map[string]any)
		require.Equal(t, name, md["name"])
	}

	modified := makeObject("default", "cm-a", "12", gvk)
	fakeWatcher.Modify(modified)

	evt := receiveEvent[resolver.SubscriptionEnvelope](t, ctx, ch)
	require.Equal(t, resolver.EventTypeModified, evt.Type)
	obj := evt.Object.(map[string]any)
	md := obj["metadata"].(map[string]any)
	require.Equal(t, "cm-a", md["name"])

	fakeWatcher.Stop()
}

func TestSubscribeItems_WithResourceVersion_SkipsList_WatchesFromProvidedRV(t *testing.T) {
	log := testlogger.New().Logger

	mockClient := &commonmocks.MockWithWatch{}
	defer mockClient.AssertExpectations(t)

	gvk := schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "ConfigMap"}

	fakeWatcher := watch.NewFake()
	mockClient.On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(fakeWatcher, nil).
		Once()

	svc := resolver.New(log, mockClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := map[string]any{
		resolver.SubscribeToAllArg:  true,
		resolver.ResourceVersionArg: "999",
	}
	p := graphql.ResolveParams{Context: ctx, Args: args}

	field := svc.SubscribeItems(gvk, apiextensionsv1.ClusterScoped)
	chAny, err := field(p)
	require.NoError(t, err)
	ch := chAny.(chan any)

	added := makeObject("default", "cm-x", "1000", gvk)
	fakeWatcher.Add(added)

	evt := receiveEvent[resolver.SubscriptionEnvelope](t, ctx, ch)
	require.Equal(t, resolver.EventTypeAdded, evt.Type)
	obj := evt.Object.(map[string]any)
	md := obj["metadata"].(map[string]any)
	require.Equal(t, "cm-x", md["name"])

	fakeWatcher.Stop()
}

func TestSubscribeItems_DeletedEvent_CarriesObject(t *testing.T) {
	log := testlogger.New().Logger

	mockClient := &commonmocks.MockWithWatch{}
	defer mockClient.AssertExpectations(t)

	gvk := schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "ConfigMap"}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			lst := args[1].(*unstructured.UnstructuredList)
			lst.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
			lst.SetResourceVersion("rv0")
			lst.Items = []unstructured.Unstructured{}
		}).
		Return(nil).
		Once()

	fakeWatcher := watch.NewFake()
	mockClient.On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(fakeWatcher, nil).
		Once()

	svc := resolver.New(log, mockClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := map[string]any{resolver.SubscribeToAllArg: true}
	p := graphql.ResolveParams{Context: ctx, Args: args}

	field := svc.SubscribeItems(gvk, apiextensionsv1.ClusterScoped)
	chAny, err := field(p)
	require.NoError(t, err)
	ch := chAny.(chan any)

	deleted := makeObject("default", "cm-del", "101", gvk)
	fakeWatcher.Delete(deleted)

	evt := receiveEvent[resolver.SubscriptionEnvelope](t, ctx, ch)
	require.Equal(t, resolver.EventTypeDeleted, evt.Type)
	obj := evt.Object.(map[string]any)
	md := obj["metadata"].(map[string]any)
	require.Equal(t, "cm-del", md["name"])
	require.NotEmpty(t, md["resourceVersion"])

	fakeWatcher.Stop()
}

func TestSubscribeItems_InvalidLabelSelector_EmitsErrorAndCloses(t *testing.T) {
	log := testlogger.New().Logger

	mockClient := &commonmocks.MockWithWatch{}
	defer mockClient.AssertExpectations(t)

	gvk := schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "ConfigMap"}

	svc := resolver.New(log, mockClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := map[string]any{
		resolver.SubscribeToAllArg: true,
		resolver.LabelSelectorArg:  "invalid==(selector",
	}
	p := graphql.ResolveParams{Context: ctx, Args: args}

	field := svc.SubscribeItems(gvk, apiextensionsv1.ClusterScoped)
	chAny, err := field(p)
	require.NoError(t, err)
	ch := chAny.(chan any)

	evtErr := receiveEvent[error](t, ctx, ch)
	require.Error(t, evtErr)
	require.Contains(t, evtErr.Error(), "invalid label selector")
}

func TestSubscribeItems_WatchStartError_EmitsError(t *testing.T) {
	log := testlogger.New().Logger

	mockClient := &commonmocks.MockWithWatch{}
	defer mockClient.AssertExpectations(t)

	gvk := schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "ConfigMap"}

	mockClient.On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("watch-start-error")).
		Once()

	svc := resolver.New(log, mockClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := map[string]any{
		resolver.SubscribeToAllArg:  true,
		resolver.ResourceVersionArg: "42",
	}
	p := graphql.ResolveParams{Context: ctx, Args: args}

	field := svc.SubscribeItems(gvk, apiextensionsv1.ClusterScoped)
	chAny, err := field(p)
	require.NoError(t, err)
	ch := chAny.(chan any)

	evtErr := receiveEvent[error](t, ctx, ch)
	require.Error(t, evtErr)
	require.Contains(t, evtErr.Error(), "failed to start watch")
}

func TestSubscribeItems_WatchEmitsWrongObjectType_EmitsError(t *testing.T) {
	log := testlogger.New().Logger

	mockClient := &commonmocks.MockWithWatch{}
	defer mockClient.AssertExpectations(t)

	gvk := schema.GroupVersionKind{Group: "core", Version: "v1", Kind: "ConfigMap"}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			lst := args[1].(*unstructured.UnstructuredList)
			lst.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
			lst.SetResourceVersion("rv-list-xyz")
			lst.Items = []unstructured.Unstructured{}
		}).
		Return(nil).
		Once()

	fakeWatcher := watch.NewFake()
	mockClient.On("Watch", mock.Anything, mock.Anything, mock.Anything).
		Return(fakeWatcher, nil).
		Once()

	svc := resolver.New(log, mockClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := map[string]any{resolver.SubscribeToAllArg: true}
	p := graphql.ResolveParams{Context: ctx, Args: args}

	field := svc.SubscribeItems(gvk, apiextensionsv1.ClusterScoped)
	chAny, err := field(p)
	require.NoError(t, err)
	ch := chAny.(chan any)

	fakeWatcher.Action(watch.Added, &unstructured.UnstructuredList{})

	evtErr := receiveEvent[error](t, ctx, ch)
	require.Error(t, evtErr)
	require.Contains(t, evtErr.Error(), "failed to cast event object to unstructured")

	fakeWatcher.Stop()
}

func makeObject(ns, name, rv string, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"metadata": map[string]any{
			"name":            name,
			"namespace":       ns,
			"resourceVersion": rv,
		},
	}}
	return obj
}

func receiveEvent[T any](t *testing.T, ctx context.Context, ch <-chan any) T {
	t.Helper()

	select {
	case <-ctx.Done():
		t.Fatalf("context cancelled before receiving event: %v", ctx.Err())
	case v := <-ch:
		ev, ok := v.(T)
		if !ok {
			t.Fatalf("received value of wrong type: got %T, want %T", v, *new(T))
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for event")
	}

	var zero T
	return zero
}
