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
	measureMetricsRejected(count int)
	measureConversionDuration(duration time.Duration)
}

func NewRecorder(reg prometheus.Registerer) Recorder {
	r := &prometheusRecorder{
		proxyMetricsParsed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "metrics_parsed_total",
			Help:      "The total number of metrics that have been parsed.",
		}, []string{}),
		proxyMetricsRejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "metrics_rejected_total",
			Help:      "The total number of metrics that were rejected.",
		}, []string{}),
		conversionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: prefix,
			Name:      "data_conversion_seconds",
			Help:      "Time (in seconds) spent converting ingested InfluxDB data into Prometheus data.",
			Buckets:   instrument.DefBuckets,
		}, []string{}),
	}

	reg.MustRegister(r.proxyMetricsParsed, r.proxyMetricsRejected, r.conversionDuration)

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for
// Prometheus.
type prometheusRecorder struct {
	proxyMetricsParsed   *prometheus.CounterVec
	proxyMetricsRejected *prometheus.CounterVec
	conversionDuration   *prometheus.HistogramVec
}

// measureMetricsParsed measures the total amount of received points on Prometheus.
func (r prometheusRecorder) measureMetricsParsed(count int) {
	r.proxyMetricsParsed.WithLabelValues().Add(float64(count))
}

// measureRejectedmetrics measures the total amount of rejected points on Prometheus.
func (r prometheusRecorder) measureMetricsRejected(count int) {
	r.proxyMetricsRejected.WithLabelValues().Add(float64(count))
}

// measureConversionDuration measures the total time spent translating points to Prometheus format
func (r prometheusRecorder) measureConversionDuration(duration time.Duration) {
	r.conversionDuration.WithLabelValues().Observe(duration.Seconds())
}
