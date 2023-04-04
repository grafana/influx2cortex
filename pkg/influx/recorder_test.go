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
	CommitUnixTimestamp = "1649113429"
	DockerTag = "docker:tag"

	reg := prometheus.NewRegistry()
	rec := NewRecorder(reg)

	tests := map[string]struct {
		measure        func(r Recorder)
		expMetricNames []string
		expMetrics     string
	}{
		"Measure incoming points": {
			measure: func(r Recorder) {
				r.measureMetricsParsed(3)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_metrics_parsed_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_metrics_parsed_total The total number of metrics that have been parsed.
# TYPE influxdb_proxy_ingester_metrics_parsed_total counter
influxdb_proxy_ingester_metrics_parsed_total 3
`,
		},
		"Measure dropped samples": {
			measure: func(r Recorder) {
				r.measureMetricsDropped(1)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_metrics_dropped_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_metrics_dropped_total The total number of metrics that have been dropped.
# TYPE influxdb_proxy_ingester_metrics_dropped_total counter
influxdb_proxy_ingester_metrics_dropped_total 1
`,
		},
		"Measure rejected samples": {
			measure: func(r Recorder) {
				r.measureProxyErrors("reason")
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_proxy_errors_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_proxy_errors_total The total number of errors, sliced by the go error type returned.
# TYPE influxdb_proxy_ingester_proxy_errors_total counter
influxdb_proxy_ingester_proxy_errors_total{reason="reason"} 1
`,
		},
		"Measure written samples": {
			measure: func(r Recorder) {
				r.measureMetricsWritten(3)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_metrics_written_total",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_metrics_written_total The total number of metrics that have been written.
# TYPE influxdb_proxy_ingester_metrics_written_total counter
influxdb_proxy_ingester_metrics_written_total 3
`,
		},
		"Measure conversion duration": {
			measure: func(r Recorder) {
				r.measureConversionDuration(15 * time.Second)
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_data_conversion_seconds",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_data_conversion_seconds Time (in seconds) spent converting ingested InfluxDB data into Prometheus data.
# TYPE influxdb_proxy_ingester_data_conversion_seconds histogram
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.005"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.01"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.025"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.05"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.1"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.25"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="0.5"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="1"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="2.5"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="5"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="10"} 0
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="25"} 1
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="50"} 1
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="100"} 1
influxdb_proxy_ingester_data_conversion_seconds_bucket{le="+Inf"} 1
influxdb_proxy_ingester_data_conversion_seconds_sum 15
influxdb_proxy_ingester_data_conversion_seconds_count 1
`,
		},
		"Register version build timestamp": {
			measure: func(r Recorder) {
				_ = r.RegisterVersionBuildTimestamp()
			},
			expMetricNames: []string{
				"influxdb_proxy_ingester_build_unix_timestamp",
			},
			expMetrics: `
# HELP influxdb_proxy_ingester_build_unix_timestamp A constant build date value reported by each instance as a Unix epoch timestamp
# TYPE influxdb_proxy_ingester_build_unix_timestamp gauge
influxdb_proxy_ingester_build_unix_timestamp{docker_tag="docker:tag"} 1.649113429e+09
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
