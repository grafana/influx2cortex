module github.com/grafana/influx2cortex

go 1.15

require (
	github.com/cortexproject/cortex v1.6.0
	github.com/go-kit/kit v0.10.0
	github.com/influxdata/influxdb/v2 v2.0.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/prometheus v2.5.0+incompatible
	github.com/sirupsen/logrus v1.7.0
	github.com/weaveworks/common v0.0.0-20210112142934-23c8d7fa6120
	google.golang.org/grpc v1.35.0
)

// We can't upgrade to grpc 1.30.0 until go.etcd.io/etcd will support it.
replace google.golang.org/grpc => google.golang.org/grpc v1.29.1
