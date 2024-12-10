package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

const MAX_CONNECTIONS = 10

func getArgs() Chord {
	address := flag.String("a", "localhost", "Address of chord client")
	portvar := flag.String("p", "1234", "a port")

	joinAddress := flag.String("ja", "XXX", "Address of node in chord ring")
	joinPort := flag.String("jp", "2222", "Port of node in chord ring")

	stabilize := flag.Int("ts", 2000, "Interval between stabilize")
	fixFingers := flag.Int("tff", 2000, "Interval between fix fingers")
	checkPred := flag.Int("tcp", 2000, "Interval between check predecessors")

	identifier := flag.String("i", "XXX", "Identifier")
	numSucc := flag.Int("r", 2, "Number of successors")
	verbose := flag.Bool("v", false, "a bool")
	flag.Parse()

	if !verifyRange(*stabilize, *fixFingers, *checkPred) {
		log.Fatalf("Arguments for --ts --tff --tcp not in range")
	}

	nodeAddress := *address + ":" + *portvar
	id := getIdentifier(nodeAddress, *identifier)
	return Chord{"tcp",
		NodeInfo{*id, nodeAddress, "", make([]string, 0)},
		*numSucc, *joinAddress, *joinPort,
		Timings{
			time.Millisecond * time.Duration(*stabilize),
			time.Millisecond * time.Duration(*fixFingers),
			time.Millisecond * time.Duration(*checkPred),
		},
		ChordTracer{*verbose},
		sync.Mutex{},
	}
}

func (tracer ChordTracer) Trace(format string, a ...any) {
	if tracer.verbose {
		fmt.Printf(format+"\n", a...)
	}
}

func (chord *Chord) Create() {
	chord.tracer.Trace("Node %s: I am creating chord ring!", chord.node.NodeAddress)

	// Create ring: The own node is both successor and predecessor
	chord.node.Predecessor = chord.node.NodeAddress
	chord.node.Successors = append(chord.node.Successors, chord.node.NodeAddress)

}

func (chord *Chord) Join(joinReq *JoinRequest, joinResponse *JoinResponse) error {
	// Add joining node to ring
	fmt.Printf("Node: %s is trying to join\n", joinReq.IpAddressAndPort)
	// TODO:query ring to get correct successor node
	// Fetch from finger table
	joinResponse.SuccessorNode = "Snoppen:2133"
	joinResponse.SuccessorNodeIdentifier = "AAA"
	return nil
}

func (chord *Chord) CallJoin() {
	joinReq := JoinRequest{chord.node.NodeAddress, chord.node.Identifier.String()}
	JoinResponse := JoinResponse{}
	if !call("Chord.Join", &joinReq, &JoinResponse, chord.joinNodeIp+":"+chord.joinNodePort) {
		log.Fatal("Cannot join specified node on chord ring")
	}
	// Join successful
	chord.node.Successors = append(chord.node.Successors, JoinResponse.SuccessorNode)
	// Notify successor

}

func main() {
	chord := getArgs()

	chord.rpcListener()
	fmt.Printf("Chord started: listening on: %s\n", chord.node.NodeAddress)
	printHash(&chord.node.Identifier)
	if chord.joinNodeIp == "XXX" {
		chord.Create()
	} else {
		chord.CallJoin()
	}

	for {
		time.Sleep(time.Second)
	}
}

// start a thread that listens for RPCs from worker.go
func (chord *Chord) rpcListener() {
	rpc.Register(chord)
	rpc.HandleHTTP()

	chord.tracer.Trace("Rpc handler listening on port %s", strings.Split(chord.node.NodeAddress, ":")[1])
	l, e := net.Listen(chord.protocol, ":"+strings.Split(chord.node.NodeAddress, ":")[1])
	// sockname := coordinatorSock()
	// os.Remove(sockname)
	// l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}
