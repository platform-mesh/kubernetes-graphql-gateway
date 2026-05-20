package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// GraphQLRequestsTotal counts GraphQL requests by cluster, operation type, and result.
	GraphQLRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubernetes_graphql_gateway_graphql_requests_total",
			Help: "Total number of GraphQL requests by cluster, operation (query/subscription), and result.",
		},
		[]string{"cluster", "operation", "result"},
	)

	// GraphQLRequestDuration observes how long each GraphQL request takes by cluster and operation.
	GraphQLRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kubernetes_graphql_gateway_graphql_request_duration_seconds",
			Help:    "Duration of GraphQL requests in seconds by cluster and operation.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"cluster", "operation"},
	)

	// KubernetesAPIRequestsTotal counts Kubernetes API calls made by the resolvers, by operation, resource kind, and result.
	KubernetesAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubernetes_graphql_gateway_kubernetes_api_requests_total",
			Help: "Total number of Kubernetes API calls by operation (list/get/create/update/delete/apply), kind, and result.",
		},
		[]string{"operation", "kind", "result"},
	)

	// KubernetesAPIRequestDuration observes how long each Kubernetes API call takes by operation and kind.
	KubernetesAPIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kubernetes_graphql_gateway_kubernetes_api_request_duration_seconds",
			Help:    "Duration of Kubernetes API calls in seconds by operation and kind.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "kind"},
	)

	// AuthRequestsTotal counts token authentication attempts by result and source.
	// result: allowed, denied, error; source: cache (served from cache), api (live TokenReview call).
	AuthRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kubernetes_graphql_gateway_auth_requests_total",
			Help: "Total number of token authentication attempts by result (allowed/denied/error) and source (cache/api).",
		},
		[]string{"result", "source"},
	)

	// AuthRequestDuration observes how long live TokenReview API calls take.
	AuthRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kubernetes_graphql_gateway_auth_request_duration_seconds",
			Help:    "Duration of token authentication calls in seconds by source (cache/api).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"},
	)

)

func init() {
	prometheus.MustRegister(
		GraphQLRequestsTotal,
		GraphQLRequestDuration,
		KubernetesAPIRequestsTotal,
		KubernetesAPIRequestDuration,
		AuthRequestsTotal,
		AuthRequestDuration,
	)
}
