package main

import (
	"crypto/x509"
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

	//TLS info
	trustedCerts x509.CertPool

	// Stored files
	files           []string
	replicatedFiles []string

	// Sync and information
	tracer    ChordTracer
	chordSync sync.Mutex
}

type RetCode int

const SOLONODE RetCode = 0
const MYSUCC RetCode = 1
const IAMSUCC RetCode = 2
const PASSALONG RetCode = 3

type FileStat int

const FILEOK FileStat = 0
const NOSUCHFILE FileStat = 1
const READERR FileStat = 2
const CREATEERR FileStat = 3
const WRITEERR FileStat = 4

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
