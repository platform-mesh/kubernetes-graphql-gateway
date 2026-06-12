package metrics

import "github.com/prometheus/client_golang/prometheus"

type SubscriptionMetrics struct {
	Active   prometheus.Gauge
	Total    prometheus.Counter
	Rejected prometheus.Counter
}

func NewSubscriptionMetrics(reg prometheus.Registerer) *SubscriptionMetrics {
	m := &SubscriptionMetrics{
		Active: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "graphql_subscriptions_active",
			Help: "Current number of active (in-flight) GraphQL subscriptions.",
		}),
		Total: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "graphql_subscriptions_total",
			Help: "Total number of GraphQL subscriptions opened.",
		}),
		Rejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "graphql_subscriptions_rejected_total",
			Help: "Total number of GraphQL subscriptions rejected due to max-inflight limit.",
		}),
	}
	reg.MustRegister(m.Active, m.Total, m.Rejected)
	return m
}

// EndpointMetrics tracks GraphQL requests per cluster endpoint.
type EndpointMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

// NewEndpointMetrics creates and registers endpoint metrics.
func NewEndpointMetrics(reg prometheus.Registerer) *EndpointMetrics {
	m := &EndpointMetrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kubernetes_graphql_gateway_graphql_requests_total",
			Help: "Total number of GraphQL requests by cluster, operation (query/subscription), and result.",
		}, []string{"cluster", "operation", "result"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kubernetes_graphql_gateway_graphql_request_duration_seconds",
			Help:    "Duration of GraphQL requests in seconds by cluster and operation.",
			Buckets: prometheus.DefBuckets,
		}, []string{"cluster", "operation"}),
	}
	reg.MustRegister(m.RequestsTotal, m.RequestDuration)
	return m
}

// ResolverMetrics tracks Kubernetes API calls made by GraphQL resolvers.
type ResolverMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

// NewResolverMetrics creates and registers resolver metrics.
func NewResolverMetrics(reg prometheus.Registerer) *ResolverMetrics {
	m := &ResolverMetrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kubernetes_graphql_gateway_kubernetes_api_requests_total",
			Help: "Total number of Kubernetes API calls by operation (list/get/create/update/delete/apply), kind, and result.",
		}, []string{"operation", "kind", "result"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kubernetes_graphql_gateway_kubernetes_api_request_duration_seconds",
			Help:    "Duration of Kubernetes API calls in seconds by operation and kind.",
			Buckets: prometheus.DefBuckets,
		}, []string{"operation", "kind"}),
	}
	reg.MustRegister(m.RequestsTotal, m.RequestDuration)
	return m
}

// AuthMetrics tracks token authentication attempts.
type AuthMetrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

// NewAuthMetrics creates and registers auth metrics.
func NewAuthMetrics(reg prometheus.Registerer) *AuthMetrics {
	m := &AuthMetrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kubernetes_graphql_gateway_auth_requests_total",
			Help: "Total number of token authentication attempts by result (allowed/denied/error) and source (cache/api).",
		}, []string{"result", "source"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kubernetes_graphql_gateway_auth_request_duration_seconds",
			Help:    "Duration of token authentication calls in seconds by source (cache/api).",
			Buckets: prometheus.DefBuckets,
		}, []string{"source"}),
	}
	reg.MustRegister(m.RequestsTotal, m.RequestDuration)
	return m
}
