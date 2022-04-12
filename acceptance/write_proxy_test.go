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
		ResultType string `json:"metric"`
		Result     []struct {
			Metric model.Metric  `json:"metric"`
			Value  []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (s *Suite) Test_WriteLineProtocol() {
	s.waitUntilElapsedAfterSuiteSetup(5 * time.Second)

	line := fmt.Sprintf("stat,unit=temperature avg=%f,max=%f", 23.5, 45.0)
	err := s.api.writeAPI.WriteRecord(context.Background(), line)
	s.Require().NoError(err)

	code, resp, err := s.api.proxy_client.query("prometheus/api/v1/query?query=stat_avg", "unknown")
	s.Require().NoError(err)
	s.Require().Equal(200, code)

	expectedMetric := model.Metric{
		model.MetricNameLabel:               "stat_avg",
		model.LabelName("__proxy_source__"): "influx",
		model.LabelName("unit"):             "temperature",
	}

	var writeResponse Response
	err = json.Unmarshal(resp, &writeResponse)
	s.Require().NoError(err)

	s.Require().Equal(expectedMetric.Equal(writeResponse.Data.Result[0].Metric), true)
}
