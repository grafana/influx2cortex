package main

import (
	"fmt"
	"os"

	"github.com/grafana/influx2cortex/pkg/influx"
)

func main() {
	err := influx.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running influx2cortex: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
