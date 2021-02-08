module github.com/grafana/influx2cortex

go 1.15

require (
	github.com/cortexproject/cortex v1.6.1-0.20210129172402-0976147451ee
	github.com/go-kit/kit v0.10.0
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/influxdata/influxdb/v2 v2.0.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/prometheus v1.8.2-0.20210124145330-b5dfa2414b9e
	github.com/sirupsen/logrus v1.7.0
	github.com/weaveworks/common v0.0.0-20210112142934-23c8d7fa6120
	google.golang.org/grpc v1.35.0
)

// We can't upgrade to grpc 1.30.0 until go.etcd.io/etcd will support it.
replace google.golang.org/grpc => google.golang.org/grpc v1.29.1

// Replacing this cuts that dependency branch by making an older cortex version depend on on a newer thanos
replace github.com/thanos-io/thanos v0.13.1-0.20210108102609-f85e4003ba51 => github.com/thanos-io/thanos v0.13.1-0.20210122144644-4b4994212b24

exclude (
	// Exclude pre-go-mod kubernetes tags, as they are older
	// than v0.x releases but are picked when we update the dependencies.
	k8s.io/client-go v1.4.0
	k8s.io/client-go v1.4.0+incompatible
	k8s.io/client-go v1.5.0
	k8s.io/client-go v1.5.0+incompatible
	k8s.io/client-go v1.5.1
	k8s.io/client-go v1.5.1+incompatible
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/client-go v2.0.0+incompatible
	k8s.io/client-go v2.0.0-alpha.1+incompatible
	k8s.io/client-go v3.0.0+incompatible
	k8s.io/client-go v3.0.0-beta.0+incompatible
	k8s.io/client-go v4.0.0+incompatible
	k8s.io/client-go v4.0.0-beta.0+incompatible
	k8s.io/client-go v5.0.0+incompatible
	k8s.io/client-go v5.0.1+incompatible
	k8s.io/client-go v6.0.0+incompatible
	k8s.io/client-go v7.0.0+incompatible
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/client-go v9.0.0-invalid+incompatible
)
