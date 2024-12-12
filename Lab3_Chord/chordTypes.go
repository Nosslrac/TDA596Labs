package main

import (
	"math/big"
	"sync"
	"time"
)

type Chord struct {
	// Communication protocol
	protocol string

	// Chord specific info
	node    NodeInfo
	numSucc int

	// Existing node in chord ring
	joinNodeIp   string
	joinNodePort string

	// Chord specific timings
	intervalTimings Timings

	// Sync and information
	tracer    ChordTracer
	chordSync sync.Mutex
}

type RetCode int

const SOLONODE RetCode = 0
const ISSUCC RetCode = 1
const PASSALONG RetCode = 2

type FingerEntry struct {
	Identifier  big.Int
	NodeAddress NodeAddress
}

type NodeAddress string

type NodeInfo struct {
	Identifier  big.Int
	NodeAddress NodeAddress
	Predecessor NodeAddress
	FingerTable []FingerEntry
	Successors  []NodeAddress
	// Add more
}

type Timings struct {
	Stabilize  time.Duration
	FixFingers time.Duration
	CheckPred  time.Duration
}

type ChordTracer struct {
	verbose bool
}
