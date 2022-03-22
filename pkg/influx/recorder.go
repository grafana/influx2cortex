package influx

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
)

const (
	prefix = "influxdb_proxy_ingester"
)

//go:generate mockery --inpackage --testonly --case underscore --name Recorder
type Recorder interface {
	measureMetricsParsed(count int)
	measureMetricsWritten(count int)
	measureProxyErrors(reason string)
	measureConversionDuration(duration time.Duration)
}

func NewRecorder(reg prometheus.Registerer) Recorder {
	r := &prometheusRecorder{
		proxyMetricsParsed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "metrics_parsed_total",
			Help:      "The total number of metrics that have been parsed.",
		}, []string{}),
		proxyErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "proxy_errors_total",
			Help:      "The total number of errors, sliced by the go error type returned.",
		}, []string{"reason"}),
		proxyMetricsWritten: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "metrics_written_total",
			Help:      "The total number of metrics that have been written.",
		}, []string{}),
		conversionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: prefix,
			Name:      "data_conversion_seconds",
			Help:      "Time (in seconds) spent converting ingested InfluxDB data into Prometheus data.",
			Buckets:   instrument.DefBuckets,
		}, []string{}),
	}

	reg.MustRegister(r.proxyMetricsParsed, r.proxyMetricsWritten, r.proxyErrors, r.conversionDuration)

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for
// Prometheus.
type prometheusRecorder struct {
	proxyMetricsParsed  *prometheus.CounterVec
	proxyMetricsWritten *prometheus.CounterVec
	proxyErrors         *prometheus.CounterVec
	conversionDuration  *prometheus.HistogramVec
}

// measureMetricsParsed measures the total amount of metrics parsed by the proxy.
func (r prometheusRecorder) measureMetricsParsed(count int) {
	r.proxyMetricsParsed.WithLabelValues().Add(float64(count))
}

// measureMetricsParsed measures the total amount of metrics written.
func (r prometheusRecorder) measureMetricsWritten(count int) {
	r.proxyMetricsWritten.WithLabelValues().Add(float64(count))
}

// measureProxyErrors measures the total amount of errors encountered.
func (r prometheusRecorder) measureProxyErrors(reason string) {
	r.proxyErrors.WithLabelValues(reason).Add(1)
}

// measureConversionDuration measures the total time spent translating points to Prometheus format
func (r prometheusRecorder) measureConversionDuration(duration time.Duration) {
	r.conversionDuration.WithLabelValues().Observe(duration.Seconds())
}
