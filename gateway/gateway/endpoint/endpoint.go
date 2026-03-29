package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/platform-mesh/kubernetes-graphql-gateway/apis/v1alpha1"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/authn"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/cluster"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/config"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/gateway/graphql"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/resolver"
	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/schema"
	utilscontext "github.com/platform-mesh/kubernetes-graphql-gateway/gateway/utils/context"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Endpoint combines a cluster connection with its GraphQL handler.
type Endpoint struct {
	name           string
	cluster        *cluster.Cluster
	graphqlServer  *graphql.GraphQLServer
	handler        *graphql.GraphQLHandler
	tokenValidator authn.Validator
	cancelFunc     context.CancelFunc
}

func New(
	ctx context.Context,
	name string,
	schemaJSON []byte,
	graphqlCfg config.GraphQL,
	tokenReviewCacheTTL time.Duration,
) (*Endpoint, error) {
	schemaData, err := parseSchema(schemaJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	cl, err := cluster.New(ctx, name, schemaData.ClusterMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	validatorCtx, validatorCancel := context.WithCancel(ctx)

	validator, err := authn.NewTokenReviewValidator(cl.AdminConfig(), tokenReviewCacheTTL)
	if err != nil {
		validatorCancel()
		return nil, fmt.Errorf("failed to create token validator: %w", err)
	}
	go validator.Start(validatorCtx)

	resolverProvider := resolver.New(cl.Client())
	schemaProvider, err := schema.New(ctx, schemaData.Components.Schemas, resolverProvider)
	if err != nil {
		validatorCancel()
		return nil, fmt.Errorf("failed to create GraphQL schema: %w", err)
	}

	graphqlServer := graphql.NewGraphQLServer(graphqlCfg)
	handler := graphqlServer.CreateHandler(schemaProvider.GetSchema())

	log.FromContext(ctx).Info("Registered endpoint", "cluster", name)

	return &Endpoint{
		name:           name,
		cluster:        cl,
		graphqlServer:  graphqlServer,
		handler:        handler,
		tokenValidator: validator,
		cancelFunc:     validatorCancel,
	}, nil
}

func (e *Endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e.handler == nil || e.handler.Handler == nil {
		http.Error(w, "Endpoint not ready", http.StatusServiceUnavailable)
		return
	}

	token, ok := utilscontext.GetTokenFromCtx(r.Context())
	if !ok || token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	authenticated, err := e.tokenValidator.Validate(r.Context(), token)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	if !authenticated {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle subscription requests using Server-Sent Events
	if r.Header.Get("Accept") == "text/event-stream" {
		e.graphqlServer.HandleSubscription(w, r, e.handler.Schema)
		return
	}

	e.handler.Handler.ServeHTTP(w, r)
}

func (e *Endpoint) Name() string {
	return e.name
}

func (e *Endpoint) Close() {
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
	if e.cluster != nil {
		e.cluster.Close()
	}
	e.handler = nil
	e.graphqlServer = nil
}

func parseSchema(schemaJSON []byte) (*v1alpha1.Schema, error) {
	var schemaData v1alpha1.Schema
	if err := json.Unmarshal(schemaJSON, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &schemaData, nil
}
