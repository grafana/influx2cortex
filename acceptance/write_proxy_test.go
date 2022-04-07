package influxtest

import (
	"bytes"
	"context"
	"fmt"
	"time"
)

func (s *Suite) Test_WriteLineProtocol() {
	s.waitUntilElapsedAfterSuiteSetup(10 * time.Second)

	line := fmt.Sprintf("stat,unit=temperature avg=%f,max=%f", 23.5, 45.0)
	err := s.api.writeAPI.WriteRecord(context.Background(), line)
	fmt.Println("Err: ", err)

	//code, resp, err := s.api.proxy_client.post(context.Background(), "api/v1/push/influx/write", orgId, body)
	//s.Require().NoError(err)
	//s.Require().Equal(204, code)
	//fmt.Println("Resp: ", resp)
}

func (s *Suite) Test_WriteLineProtocolInvalidCharConversion() {
	s.waitUntilElapsedAfterSuiteSetup(10 * time.Second)

	orgId := "unknown"
	body := bytes.NewReader([]byte("*measurement,#t1?=v1 f1=0 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v2/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(204, code)
}

func (s *Suite) Test_WriteLineProtocolInvalidTags() {
	s.waitUntilElapsedAfterSuiteSetup(10 * time.Second)

	//line := fmt.Sprintf("stat,unit=temperature avg=%f,max=%f", 23.5, 45.0)
	//writeAPI.WriteRecord(context.Background(), line)

	orgId := "unknown"
	body := bytes.NewReader([]byte("measurement,t1=v1,t2 f1=2 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v2/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(400, code)
}

func (s *Suite) Test_WriteLineProtocolInvalidFields() {
	//s.waitUntilElapsedAfterSuiteSetup(30 * time.Second)

	orgId := "unknown"
	body := bytes.NewReader([]byte("measurement,t1=v1 f1= 1465839830100400200"))

	code, _, err := s.api.proxy_client.post(context.Background(), "api/v2/write", orgId, body)
	s.Require().NoError(err)
	s.Require().Equal(400, code)
}
