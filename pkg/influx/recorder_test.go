package influx

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecorder(t *testing.T) {
	reg := prometheus.NewRegistry()
	rec := NewRecorder(reg)

	tests := map[string]struct {
		measure        func(r Recorder)
		expMetricNames []string
		expMetrics     string
	}{
		"Measure received points": {
			measure: func(r Recorder) {
				r.measureReceivedPoints("123", 1)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_received_points_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_received_points_total The total number of received points, excluding rejected and deduped points.
# TYPE influxdb_proxy_ingester_received_points_total counter
influxdb_proxy_ingester_received_points_total{user="123"} 1
`,
		},
		"Measure incoming points": {
			measure: func(r Recorder) {
				r.measureIncomingPoints("123", 1)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_points_in_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_points_in_total The total number of points that have come in to influx2cortex, including rejected or deduped points.
# TYPE influxdb_proxy_ingester_points_in_total counter
influxdb_proxy_ingester_points_in_total{user="123"} 1
`,
		},
		"Measure rejected samples": {
			measure: func(r Recorder) {
				r.measureRejectedPoints("123", "foo_reason")
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_rejected_points_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_rejected_points_total The total number of points that were rejected.
# TYPE influxdb_proxy_ingester_rejected_points_total counter
influxdb_proxy_ingester_rejected_points_total{reason="foo_reason", user="123"} 1
`,
		},
		"Measure conversion duration": {
			measure: func(r Recorder) {
				r.measureConversionDuration("123", 15*time.Second)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_data_conversion_seconds",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_data_conversion_seconds Time (in seconds) spent converting ingested InfluxDB data into Prometheus data.
# TYPE influxdb_proxy_ingester_data_conversion_seconds histogram
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.005"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.01"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.025"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.05"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.1"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.25"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="0.5"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="1"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="2.5"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="5"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="10"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="25"} 1
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="50"} 1
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="100"} 1
influxdb_proxy_ingester_data_conversion_seconds_bucket{user="123",le="+Inf"} 1
influxdb_proxy_ingester_data_conversion_seconds_sum{user="123"} 15
influxdb_proxy_ingester_data_conversion_seconds_count{user="123"} 1
`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Measure metrics
			test.measure(rec)

			err := testutil.GatherAndCompare(reg, strings.NewReader(test.expMetrics), test.expMetricNames...)
			assert.NoError(err)
		})
	}

}
