package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/suite"

	"github.com/platform-mesh/kubernetes-graphql-gateway/gateway/metrics"
)

type MetricsTestSuite struct {
	suite.Suite
}

func TestMetricsTestSuite(t *testing.T) {
	suite.Run(t, new(MetricsTestSuite))
}

func (s *MetricsTestSuite) TestGraphQLRequestsTotal() {
	before := testutil.ToFloat64(metrics.GraphQLRequestsTotal.WithLabelValues("my-cluster", "query", "success"))
	metrics.GraphQLRequestsTotal.WithLabelValues("my-cluster", "query", "success").Inc()
	s.Require().Equal(before+1, testutil.ToFloat64(metrics.GraphQLRequestsTotal.WithLabelValues("my-cluster", "query", "success")))

	before = testutil.ToFloat64(metrics.GraphQLRequestsTotal.WithLabelValues("my-cluster", "subscription", "error"))
	metrics.GraphQLRequestsTotal.WithLabelValues("my-cluster", "subscription", "error").Inc()
	s.Require().Equal(before+1, testutil.ToFloat64(metrics.GraphQLRequestsTotal.WithLabelValues("my-cluster", "subscription", "error")))
}

func (s *MetricsTestSuite) TestGraphQLRequestDuration() {
	before := testutil.CollectAndCount(metrics.GraphQLRequestDuration)
	metrics.GraphQLRequestDuration.WithLabelValues("my-cluster", "query").Observe(0.05)
	s.Assert().Greater(testutil.CollectAndCount(metrics.GraphQLRequestDuration), before)
}

func (s *MetricsTestSuite) TestKubernetesAPIRequestsTotal() {
	before := testutil.ToFloat64(metrics.KubernetesAPIRequestsTotal.WithLabelValues("list", "Pod", "success"))
	metrics.KubernetesAPIRequestsTotal.WithLabelValues("list", "Pod", "success").Inc()
	s.Require().Equal(before+1, testutil.ToFloat64(metrics.KubernetesAPIRequestsTotal.WithLabelValues("list", "Pod", "success")))

	before = testutil.ToFloat64(metrics.KubernetesAPIRequestsTotal.WithLabelValues("create", "Deployment", "error"))
	metrics.KubernetesAPIRequestsTotal.WithLabelValues("create", "Deployment", "error").Inc()
	s.Require().Equal(before+1, testutil.ToFloat64(metrics.KubernetesAPIRequestsTotal.WithLabelValues("create", "Deployment", "error")))
}

func (s *MetricsTestSuite) TestKubernetesAPIRequestDuration() {
	before := testutil.CollectAndCount(metrics.KubernetesAPIRequestDuration)
	metrics.KubernetesAPIRequestDuration.WithLabelValues("list", "Pod").Observe(0.1)
	s.Assert().Greater(testutil.CollectAndCount(metrics.KubernetesAPIRequestDuration), before)
}

func (s *MetricsTestSuite) TestAuthRequestsTotal() {
	before := testutil.ToFloat64(metrics.AuthRequestsTotal.WithLabelValues("allowed", "cache"))
	metrics.AuthRequestsTotal.WithLabelValues("allowed", "cache").Inc()
	s.Require().Equal(before+1, testutil.ToFloat64(metrics.AuthRequestsTotal.WithLabelValues("allowed", "cache")))

	before = testutil.ToFloat64(metrics.AuthRequestsTotal.WithLabelValues("denied", "api"))
	metrics.AuthRequestsTotal.WithLabelValues("denied", "api").Inc()
	s.Require().Equal(before+1, testutil.ToFloat64(metrics.AuthRequestsTotal.WithLabelValues("denied", "api")))
}

func (s *MetricsTestSuite) TestAuthRequestDuration() {
	before := testutil.CollectAndCount(metrics.AuthRequestDuration)
	metrics.AuthRequestDuration.WithLabelValues("api").Observe(0.02)
	s.Assert().Greater(testutil.CollectAndCount(metrics.AuthRequestDuration), before)
}

