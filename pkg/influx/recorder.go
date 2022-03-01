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
	measureReceivedPoints(user string, count int)
	measureIncomingPoints(user string, count int)
	measureRejectedPoints(user, reason string)
	measureConversionDuration(user string, duration time.Duration)
}

func NewRecorder(reg prometheus.Registerer) Recorder {
	r := &prometheusRecorder{
		receivedPoints: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "received_points_total",
			Help:      "The total number of received points, excluding rejected and deduped points.",
		}, []string{"user"}),
		incomingPoints: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "points_in_total",
			Help: "The total number of points that have come in to influx2cortex, including rejected " +
				"or deduped points.",
		}, []string{"user"}),
		rejectedPoints: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: prefix,
			Name:      "rejected_points_total",
			Help:      "The total number of points that were rejected.",
		}, []string{"user", "reason"}),
		conversionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: prefix,
			Name:      "data_conversion_seconds",
			Help:      "Time (in seconds) spent converting ingested InfluxDB data into Prometheus data.",
			Buckets:   instrument.DefBuckets,
		}, []string{"user"}),
	}

	reg.MustRegister(r.receivedPoints, r.incomingPoints, r.rejectedPoints, r.conversionDuration)

	return r
}

// prometheusRecorder knows the metrics of the ingester and how to measure them for
// Prometheus.
type prometheusRecorder struct {
	receivedPoints     *prometheus.CounterVec
	incomingPoints     *prometheus.CounterVec
	rejectedPoints     *prometheus.CounterVec
	conversionDuration *prometheus.HistogramVec
}

// measureMetricsParsed measures the total amount of received points on Prometheus.
func (r prometheusRecorder) measureReceivedPoints(user string, count int) {
	r.receivedPoints.WithLabelValues(user).Add(float64(count))
}

// measureIncomingmetrics measures the total amount of incoming points on Prometheus.
func (r prometheusRecorder) measureIncomingPoints(user string, count int) {
	r.incomingPoints.WithLabelValues(user).Add(float64(count))
}

// measureRejectedmetrics measures the total amount of rejected points on Prometheus.
func (r prometheusRecorder) measureRejectedPoints(user, reason string) {
	r.rejectedPoints.WithLabelValues(user, reason).Add(1)
}

// measureConversionDuration measures the total time spent translating points to Prometheus format
func (r prometheusRecorder) measureConversionDuration(user string, duration time.Duration) {
	r.conversionDuration.WithLabelValues(user).Observe(duration.Seconds())
}
