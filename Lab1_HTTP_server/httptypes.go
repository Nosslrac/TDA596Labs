package main

import "sync"

type HTTPserver struct {
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
