package main

import "sync"

type HTTPserver struct {
	port            string
	serverType      string
	numConnections  int
	numTotal        int
	serverCondition sync.Cond
}
