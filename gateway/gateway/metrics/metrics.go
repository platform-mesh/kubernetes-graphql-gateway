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
