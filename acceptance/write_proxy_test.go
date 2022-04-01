package influxtest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/prometheus/common/model"
)

type Response struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resulttype"`
		Result     []struct {
			Metric model.Metric     `json:"metric"`
			Sample model.SamplePair `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

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

	s.verifyCortexWrite("prometheus/api/v1/query?query=stat_avg", expectedResult)
}

func (s *Suite) Test_WriteLineProtocol_MultipleFields() {
	line := fmt.Sprintf("measurement,t1=v1 f1=2,f2=3")
	err := s.api.writeAPI.WriteRecord(context.Background(), line)
	s.Require().NoError(err)

	expectedResultLine1 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "measurement_f1",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("t1"):               "v1",
		},
		Value: 2,
	}
	expectedResultLine2 := model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel:               "measurement_f2",
			model.LabelName("__proxy_source__"): "influx",
			model.LabelName("t1"):               "v1",
		},
		Value: 3,
	}

	s.verifyCortexWrite("prometheus/api/v1/query?query=measurement_f1", expectedResultLine1)
	s.verifyCortexWrite("prometheus/api/v1/query?query=measurement_f2", expectedResultLine2)

}

func (s *Suite) verifyCortexWrite(path string, expectedResult model.Sample) {
	code, resp, err := s.api.proxy_client.query(path, "unknown")
	s.Require().NoError(err)
	s.Require().Equal(200, code)

	var writeResponse Response
	err = json.Unmarshal(resp, &writeResponse)
	s.Require().NoError(err)

	s.Require().Equal(expectedResult.Metric.Equal(writeResponse.Data.Result[0].Metric), true)
	s.Require().Equal(expectedResult.Value, writeResponse.Data.Result[0].Sample.Value)
}
