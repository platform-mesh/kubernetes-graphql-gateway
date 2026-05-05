package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSubscriptionMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewSubscriptionMetrics(reg)

	families, err := reg.Gather()
	require.NoError(t, err)

	names := make(map[string]struct{})
	for _, f := range families {
		names[f.GetName()] = struct{}{}
	}
	assert.Contains(t, names, "graphql_subscriptions_active")
	assert.Contains(t, names, "graphql_subscriptions_total")
	assert.Contains(t, names, "graphql_subscriptions_rejected_total")

	assert.Equal(t, 0.0, gaugeValue(t, m.Active))
	assert.Equal(t, 0.0, counterValue(t, m.Total))
	assert.Equal(t, 0.0, counterValue(t, m.Rejected))
}

func TestSubscriptionMetricsIncDec(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewSubscriptionMetrics(reg)

	m.Active.Inc()
	m.Active.Inc()
	m.Total.Inc()
	m.Total.Inc()
	m.Total.Inc()
	m.Rejected.Inc()

	assert.Equal(t, 2.0, gaugeValue(t, m.Active))
	assert.Equal(t, 3.0, counterValue(t, m.Total))
	assert.Equal(t, 1.0, counterValue(t, m.Rejected))

	m.Active.Dec()
	assert.Equal(t, 1.0, gaugeValue(t, m.Active))
}

func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, g.Write(&m))
	return m.GetGauge().GetValue()
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}
