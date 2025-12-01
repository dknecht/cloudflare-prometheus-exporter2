package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// -------- Test: BuildAllMetricsSet --------
func TestBuildAllMetricsSet(t *testing.T) {
	metricsSet := BuildAllMetricsSet()

	assert.True(t, metricsSet.Has("cloudflare_zone_requests_total"))
	assert.True(t, metricsSet.Has("cloudflare_zone_bandwidth_total"))
	assert.False(t, metricsSet.Has("non_existent_metric"))
}

// -------- Test: BuildDeniedMetricsSet --------
func TestBuildDeniedMetricsSet_ValidMetrics(t *testing.T) {
	metricsList := []string{"cloudflare_zone_requests_total", "cloudflare_zone_threats_total"}
	set, err := BuildDeniedMetricsSet(metricsList)

	assert.NoError(t, err)
	assert.True(t, set.Has("cloudflare_zone_requests_total"))
	assert.True(t, set.Has("cloudflare_zone_threats_total"))
	assert.False(t, set.Has("cloudflare_zone_bandwidth_total"))
}

func TestBuildDeniedMetricsSet_InvalidMetric(t *testing.T) {
	metricsList := []string{"non_existent_metric"}
	set, err := BuildDeniedMetricsSet(metricsList)

	assert.Error(t, err)
	assert.Nil(t, set)
}

// -------- Test: getLabels --------
func Test_getLabels_WithHost(t *testing.T) {
	viper.Set("exclude_host", false)
	base := prometheus.Labels{"zone": "example", "account": "abc"}
	result := getLabels(base, "test-host")

	assert.Equal(t, "test-host", result["host"])
	assert.Equal(t, "example", result["zone"])
	assert.Equal(t, "abc", result["account"])
}

func Test_getLabels_WithoutHost(t *testing.T) {
	viper.Set("exclude_host", true)
	base := prometheus.Labels{"zone": "example", "account": "abc"}
	result := getLabels(base, "test-host")

	_, exists := result["host"]
	assert.False(t, exists)
}

// -------- Test: MustRegisterMetrics (basic test) --------
func TestMustRegisterMetrics_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Expected no panic in MustRegisterMetrics, but got: %v", r)
		}
	}()
	denied := Set{} // empty set = allow all
	MustRegisterMetrics(denied)
}
