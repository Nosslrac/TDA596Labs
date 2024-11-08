package main

import (
	"fmt"
	"sync"
)

type HTTPProxy struct {
	port            string
	serverType      string
	numConnections  int
	numTotal        int
	tracer          HTTPTracer
	serverCondition sync.Cond
}

type HTTPTracer struct {
	verbose bool
}

func (tracer HTTPTracer) Trace(format string, a ...any) {
	if tracer.verbose {
		fmt.Printf(format+"\n", a...)
	}
}
