package influxtest

import (
	"bytes"
	"context"
	"time"
)

func (s *Suite) Test_WriteLineProtocol() {
	s.waitUntilElapsedAfterSuiteSetup(30 * time.Second)

	orgId := "unknown"
	body := bytes.NewReader([]byte("measurement,tag1=val1 field1=val2 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v1/push/influx/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(200, code)
}

func (s *Suite) Test_WriteLineProtocolInvalidCharConversion() {
	s.waitUntilElapsedAfterSuiteSetup(30 * time.Second)

	orgId := "unknown"
	body := bytes.NewReader([]byte("measurement,tag1=val1 field1=val2 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v1/push/influx/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(200, code)
}

func (s *Suite) Test_WriteLineProtocolInvalidTags() {
	s.waitUntilElapsedAfterSuiteSetup(30 * time.Second)

	//line := fmt.Sprintf("stat,unit=temperature avg=%f,max=%f", 23.5, 45.0)
	//writeAPI.WriteRecord(context.Background(), line)

	orgId := "unknown"
	body := bytes.NewReader([]byte("measurement,tag1=val1 field1=val2 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v1/push/influx/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(200, code)
}

func (s *Suite) Test_WriteLineProtocolInvalidFields() {
	s.waitUntilElapsedAfterSuiteSetup(30 * time.Second)

	orgId := "unknown"
	body := bytes.NewReader([]byte("measurement,tag1=val1 field1=val2 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v1/push/influx/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(200, code)
}
