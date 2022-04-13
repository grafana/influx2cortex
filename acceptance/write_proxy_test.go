package influxtest

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func (s *Suite) Test_WriteLineProtocol() {
	line := fmt.Sprintf("stat,unit=temperature,status=measured avg=%f", 23.5)
	err := s.api.writeAPI.WriteRecord(context.Background(), line)
	s.Require().NoError(err)

	expectedResult := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "stat_avg",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("unit"):             "temperature",
			model.LabelName("status"):           "measured",
		},
		Value: 23.5,
	}

	result, _, err := s.api.querierClient.Query(context.Background(), "stat_avg", time.Now())
	s.Require().NoError(err)

	vector := result.(model.Vector)
	s.Require().Len(vector, 1)

	s.Require().Equal(expectedResult.Metric, vector[0].Metric)
	s.Require().Equal(expectedResult.Value, vector[0].Value)
}

func (s *Suite) Test_WriteLineProtocol_MultipleFields() {
	err := s.api.writeAPI.WriteRecord(context.Background(), "measurement,t1=v1 f1=2,f2=3")
	s.Require().NoError(err)

	expectedResult1 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "measurement_f1",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("t1"):               "v1",
		},
		Value: 2,
	}
	expectedResult2 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "measurement_f2",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("t1"):               "v1",
		},
		Value: 3,
	}

	result, _, err := s.api.querierClient.Query(context.Background(), "{__name__=~\"measurement_.+\",__proxy_source__=\"influx\"}", time.Now())
	s.Require().NoError(err)
	vector := result.(model.Vector)
	s.Require().Len(vector, 2)

	s.Require().Equal(expectedResult1.Metric, vector[0].Metric)
	s.Require().Equal(expectedResult1.Value, vector[0].Value)
	s.Require().Equal(expectedResult2.Metric, vector[1].Metric)
	s.Require().Equal(expectedResult2.Value, vector[1].Value)

}

func (s *Suite) Test_WriteLineProtocol_MultipleSeries() {
	var err error
	lines := []string{
		"sample,tag1=val1 metric=3",
		"sample,tag2=val2 metric=4",
		"sample,tag3=val3 metric=5",
	}
	for _, line := range lines {
		err = s.api.writeAPI.WriteRecord(context.Background(), line)
		s.Require().NoError(err)
	}

	expectedResult1 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "sample_metric",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("tag1"):             "val1",
		},
		Value: 3,
	}
	expectedResult2 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "sample_metric",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("tag2"):             "val2",
		},
		Value: 4,
	}
	expectedResult3 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "sample_metric",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("tag3"):             "val3",
		},
		Value: 5,
	}

	result, _, err := s.api.querierClient.Query(context.Background(), "sample_metric", time.Now())
	s.Require().NoError(err)

	vector := result.(model.Vector)
	s.Require().Len(vector, 3)

	s.Require().Equal(expectedResult1.Metric, vector[0].Metric)
	s.Require().Equal(expectedResult1.Value, vector[0].Value)
	s.Require().Equal(expectedResult2.Metric, vector[1].Metric)
	s.Require().Equal(expectedResult2.Value, vector[1].Value)
	s.Require().Equal(expectedResult3.Metric, vector[2].Metric)
	s.Require().Equal(expectedResult3.Value, vector[2].Value)
}

func (s *Suite) Test_WriteLineProtocol_MultiplePoints() {
	lines := []string{
		"test_metric,test=1,tag=2 foo=1",
		"test_metric_time,test=1,tag=4 sample=3.14",
		"test_metric_duration,test=2 total=1",
	}
	for _, line := range lines {
		err := s.api.writeAPI.WriteRecord(context.Background(), line)
		s.Require().NoError(err)
	}

	expectedResult1 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "test_metric_duration_total",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("test"):             "2",
		},
		Value: 1,
	}
	expectedResult2 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "test_metric_foo",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("test"):             "1",
			model.LabelName("tag"):              "2",
		},
		Value: 1,
	}
	expectedResult3 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "test_metric_time_sample",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("test"):             "1",
			model.LabelName("tag"):              "4",
		},
		Value: 3.14,
	}

	result, _, err := s.api.querierClient.QueryRange(context.Background(),
		"{__name__=~\"test_metric_.+\",__proxy_source__=\"influx\"}",
		v1.Range{
			Start: time.Now().Add(-time.Hour),
			End:   time.Now(),
			Step:  15 * time.Second,
		})
	s.Require().NoError(err)

	matrix := result.(model.Matrix)
	s.Require().Len(matrix, 3)

	s.Require().Equal(expectedResult1.Metric, matrix[0].Metric)
	s.Require().Equal(expectedResult1.Value, matrix[0].Values[0].Value)
	s.Require().Equal(expectedResult2.Metric, matrix[1].Metric)
	s.Require().Equal(expectedResult2.Value, matrix[1].Values[0].Value)
	s.Require().Equal(expectedResult3.Metric, matrix[2].Metric)
	s.Require().Equal(expectedResult3.Value, matrix[2].Values[0].Value)
}
