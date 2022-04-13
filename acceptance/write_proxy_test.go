package influxtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
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
	err := s.api.writeAPI.WriteRecord(context.Background(), "measurement,t1=v1 f1=2,f2=3")
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

	code, resp, err := s.api.proxy_client.query("prometheus/api/v1/query?query=sample_metric", "unknown")
	s.Require().NoError(err)
	s.Require().Equal(200, code)

	var writeResponse Response
	err = json.Unmarshal(resp, &writeResponse)
	s.Require().NoError(err)
	fmt.Println("writeResponse: ", writeResponse)

	//{success {vector [{sample_metric{__proxy_source__="influx", tag2="val2"} 4 @[1649862111.057]} {sample_metric{__proxy_source__="influx", tag3="val3"} 5 @[1649862111.057]}]}}

	//s.Require().Equal(expectedResult.Metric.Equal(writeResponse.Data.Result[0].Metric), true)
	//s.Require().Equal(expectedResult.Value, writeResponse.Data.Result[0].Sample.Value)

}

func (s *Suite) Test_WriteLineProtocol_MultiplePoints() {
	//start_time := time.Now()
	lines := []string{
		"stat,unit=meters distance=534.23",
		"cpu,cpu=cpu0,status=active system_time=24657.21",
		"weather,location=us-east high_temperature=62,low_temperature=35",
	}
	for _, line := range lines {
		err := s.api.writeAPI.WriteRecord(context.Background(), line)
		s.Require().NoError(err)
	}

	code, resp, err := s.api.proxy_client.query(fmt.Sprintf(
		"prometheus/api/v1/query_range?query=%s&start=%s&end=%s&step=%s",
		url.QueryEscape("{__name__=\"~.*\",__proxy_source__=\"influx\"}"),
		FormatTime(time.Now().Add(-time.Hour)),
		FormatTime(time.Now()),
		"1s"),
		"unknown")
	s.Require().NoError(err)
	s.Require().Equal(200, code)

	fmt.Println("Code: ", code)
	fmt.Println("Resp: ", resp)
	//s.verifyCortexWrite("prometheus/api/v1/query?query=stat_distance", expectedResultLine1)
	//s.verifyCortexWrite("prometheus/api/v1/query?query=cpu_system_time", expectedResultLine2)
	//s.verifyCortexWrite("prometheus/api/v1/query?query=cpu_system_time", expectedResultLine3)
	//s.verifyCortexWrite("prometheus/api/v1/query?query=cpu_system_time", expectedResultLine4)
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

// FormatTime converts a time to a string acceptable by the Prometheus API.
func FormatTime(t time.Time) string {
	return strconv.FormatFloat(float64(t.Unix())+float64(t.Nanosecond())/1e9, 'f', -1, 64)
}
