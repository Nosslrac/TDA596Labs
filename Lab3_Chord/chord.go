package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"sync"
	"time"
)

func (tracer ChordTracer) Trace(format string, a ...any) {
	if tracer.verbose {
		fmt.Printf(format+"\n", a...)
	}
}

func (chord *Chord) parseInput(input string) {
	if input == "hash\n" {
		printHash(&chord.node.Identifier)
	} else if input == "dump\n" {
		fmt.Printf("### Node info ###\nNode identifier: %040x\nNode address: %s\nNode successors: %v\nNode predecessor: %v\nSuccessor size: %d\n\n",
			&chord.node.Identifier, chord.node.NodeAddress, chord.node.Successors, chord.node.Predecessor, len(chord.node.FingerTable))
	}

}

////////////////////////////////////////////////////
//////////////// Interval functions ////////////////
////////////////////////////////////////////////////

func (chord *Chord) insertSuccessor(succNode NodeAddress) {
	chord.node.Successors = append([]NodeAddress{succNode}, chord.node.Successors[1:]...)
}

func (chord *Chord) Create() {
	chord.tracer.Trace("Node %s: I am creating chord ring!", chord.node.NodeAddress)

	// Create ring: The own node is both successor and predecessor
	chord.node.Successors[0] = chord.node.NodeAddress
	chord.node.Predecessor = chord.node.NodeAddress

}

func (chord *Chord) FindSuccessor(findReq *FindRequest, findResp *Response) error {
	// Call succ and see if I am still its predecessor
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()

	chord.find(&findReq.Identifier, findResp)
	return nil
}

func (chord *Chord) Stabilize(stabilizeReq *StabilizeRequest, stabilizeResp *StabilizeResponse) error {
	// Call succ and see if I am still its predecessor
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()

	if chord.node.Predecessor == "X" { // Learn about our predecessor
		chord.node.Predecessor = stabilizeReq.NodeAddress
		stabilizeResp.YouGood = true
	} else if chord.node.Predecessor == stabilizeReq.NodeAddress {
		stabilizeResp.YouGood = true
	} else {
		stabilizeResp.YouGood = false
		stabilizeResp.NewPredAddress = chord.node.Predecessor
	}
	return nil
}

func (chord *Chord) FixFingers() {

	for n := 1; n <= keySize; n++ {

		identifier := chord.node.FingerTable[n].Identifier
		chord.find(&identifier, nil)
	}
}

func (chord *Chord) Notify(notifyReq *NotifyRequest, resp *NotifyResponse) error {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	chord.tracer.Trace("Got notified of new predecessor: %s", notifyReq.NodeAddress)
	chord.node.Predecessor = notifyReq.NodeAddress
	return nil
}

func (chord *Chord) Join(joinReq *JoinRequest, Response *Response) error {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	// Add joining node to ring
	fmt.Printf("Node: %s is trying to join, id: %040x\n", joinReq.NodeAddress, &joinReq.Identifier)
	// TODO:query ring to get correct successor node
	// Fetch from finger table
	// If joinReq.Identifier isBetween me & mysucc -> return mysucc

	// else
	retCode := chord.find(&joinReq.Identifier, Response)

	if retCode == MYSUCC {
		//Update all successors i.e. shift once
		chord.insertSuccessor(joinReq.NodeAddress)
	} else if retCode == SOLONODE {
		// If by myself then new node will be both pred an succ
		chord.node.Predecessor = joinReq.NodeAddress
		chord.insertSuccessor(joinReq.NodeAddress)
		chord.CallNotify(joinReq.NodeAddress)
	} else {
		chord.tracer.Trace("Checked in finger and got: %v\n", Response)
	}

	return nil
}

func (chord *Chord) find(identifier *big.Int, joinResp *Response) RetCode {
	// Find closest node
	// If finger table is not initialized yet => call successors to resolve

	// Check if solo in ring => return self
	if chord.node.Successors[0] == "X" { // SOLO
		chord.tracer.Trace("Pred in solo case")
		//Solo in ring => return ourself as succ
		joinResp.NodeAddress = chord.node.NodeAddress
		joinResp.Identifier = chord.node.Identifier //ourself
		joinResp.IsSuccessor = true
		return SOLONODE
	}

	// If between me and my successor => found the true successor
	currSucc := hashString(chord.node.Successors[0])
	if between(&chord.node.Identifier, identifier, currSucc, true) {
		chord.tracer.Trace("Pred between me and my succ")
		if currSucc.Cmp(identifier) == 0 {
			log.Fatal("Hash collision for new node")
		}
		joinResp.NodeAddress = chord.node.Successors[0]
		joinResp.Identifier = *currSucc
		joinResp.IsSuccessor = true
		return MYSUCC
	}

	return chord.closestPreceedingNode(identifier, joinResp)

}

func (chord *Chord) closestPreceedingNode(identifier *big.Int, joinResp *Response) RetCode {
	chord.tracer.Trace("Find in finger table")
	// If table is not initialized, ask successor to resolve
	if chord.node.FingerTable[1].NodeAddress == "X" {
		joinResp.IsSuccessor = false
		joinResp.NodeAddress = chord.node.Successors[0]
		return PASSALONG
	}

	for n := keySize; n > 1; n-- {
		isBetween := between(&chord.node.FingerTable[n].Identifier, identifier, &chord.node.FingerTable[n-1].Identifier, true)
		if isBetween {
			joinResp.Identifier = chord.node.FingerTable[n-1].Identifier
			joinResp.NodeAddress = chord.node.FingerTable[n-1].NodeAddress
			return PASSALONG
		}
	}
	joinResp.Identifier = chord.node.FingerTable[keySize].Identifier
	joinResp.NodeAddress = chord.node.FingerTable[keySize].NodeAddress
	return PASSALONG
}

//////////////////////////////////////////////
/////////////// RPC calls ////////////////////
//////////////////////////////////////////////

func (chord *Chord) CallNotify(nodeAddress NodeAddress) {
	notifyReq := NotifyRequest{chord.node.NodeAddress, chord.node.Identifier}
	resp := NotifyResponse{}
	if !call("Chord.Notify", &notifyReq, &resp, string(nodeAddress)) {
		log.Fatal("Cannot notify successor")
	}
}

func (chord *Chord) CallJoin(nodeAddress NodeAddress) {
	joinReq := JoinRequest{chord.node.NodeAddress, chord.node.Identifier}
	Response := Response{}
	fmt.Print(nodeAddress)
	if !call("Chord.Join", &joinReq, &Response, string(nodeAddress)) {
		log.Fatal("Cannot join specified node on chord ring")
	}
	// Join successful
	chord.tracer.Trace("### Received closest predecessor ###\nIsSucc: %v, Received: %s\nId: %040x\n",
		Response.IsSuccessor, Response.NodeAddress, &Response.Identifier)

	if Response.IsSuccessor {
		// TODO: notify node that I am new predecessor Notify node
		//Shift successors
		chord.insertSuccessor(Response.NodeAddress)
		chord.CallNotify(Response.NodeAddress)
	} else {
		// Contact new node to get successor
		chord.CallJoin(Response.NodeAddress)
	}
}

func (chord *Chord) CallFindSuccessor(nodeAddress NodeAddress) {
	joinReq := JoinRequest{chord.node.NodeAddress, chord.node.Identifier}
	Response := Response{}
	if !call("Chord.Join", &joinReq, &Response, chord.joinNodeIp+":"+chord.joinNodePort) {
		log.Fatal("Cannot join specified node on chord ring")
	}
}

func (chord *Chord) CallStabilize() {
	stabilizeReq := StabilizeRequest{chord.node.NodeAddress, chord.node.Identifier}
	stabilizeResp := StabilizeResponse{}
	if chord.node.Successors[0] == "X" {
		log.Fatal("Successor not set yet, LOGIC ERROR should ALWAYS be set after joining")
	}
	for {
		if !call("Chord.Stabilize", &stabilizeReq, &stabilizeResp, string(chord.node.Successors[0])) {
			// Our successor died: use finger table to find closest new node
			chord.tracer.Trace("Our succ has died, try to find new succ")
			break
		}
		if stabilizeResp.YouGood {
			break
		} else {
			chord.tracer.Trace("Call to stabilize: WE ARE OUTDATED, contacting new pred %s", chord.node.Predecessor)
			chord.insertSuccessor(stabilizeResp.NewPredAddress)
		}
	}

}

func main() {
	chord := getArgs()
	chord.initClient()

	chord.rpcListener()
	fmt.Printf("Chord started: node address: %v\n", chord.node.NodeAddress)
	printHash(&chord.node.Identifier)
	if chord.joinNodeIp == "XXX" {
		chord.Create()
	} else {
		chord.CallJoin(NodeAddress(chord.joinNodeIp + ":" + chord.joinNodePort))
	}

	chord.initIntervals()
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Scan line failed: %v\n", err)
		}
		chord.chordSync.Lock()
		// Test string for input
		chord.parseInput(line)
		chord.chordSync.Unlock()
	}
}

func (chord *Chord) initClient() {
	// Default init all successors to the client self
	for n := range chord.node.Successors {
		chord.node.Successors[n] = "X"
	}

	// Default init all to client self
	for n := 1; n < len(chord.node.FingerTable); n++ {
		chord.node.FingerTable[n].Identifier = *jumpIdentifier(&chord.node.Identifier, n)
		chord.node.FingerTable[n].NodeAddress = "X"
	}
	chord.node.FingerTable[0].Identifier = chord.node.Identifier
	chord.node.FingerTable[0].NodeAddress = "X"
	chord.node.Predecessor = "X"
}

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
	id := getIdentifier(NodeAddress(nodeAddress), *identifier)
	return Chord{"tcp",
		NodeInfo{*id, NodeAddress(nodeAddress), "", make([]FingerEntry, keySize+1), make([]NodeAddress, *numSucc)},
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

func (chord *Chord) initIntervals() {
	stabilizeTicker := time.NewTicker(chord.intervalTimings.Stabilize)

	// Creating channel using make
	tickerChan := make(chan bool)

	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case <-stabilizeTicker.C:
				chord.CallStabilize()
			}
		}
	}()
}
