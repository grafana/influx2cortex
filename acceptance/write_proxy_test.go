package influxtest

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	s.waitUntilElapsedAfterSuiteSetup(5 * time.Second)

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
