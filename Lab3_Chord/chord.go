package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"
)

func (tracer ChordTracer) Trace(format string, a ...any) {
	if tracer.verbose {
		fmt.Printf(format+"\n", a...)
	}
}

func (chord *Chord) dump() {
	fmt.Printf("### Node info ###\nNode identifier: %01x\nNode address: %s\nNode successors: %v\nNode predecessor: %v\n\n",
		&chord.node.Identifier, chord.node.NodeAddress, chord.node.Successors, chord.node.Predecessor)
	chord.printFingers()
}

func (chord *Chord) printFingers() {
	for n := 1; n <= 4; n++ {
		var finger *FingerEntry = &chord.node.FingerTable[n]
		d := new(big.Int).Sub(&finger.Identifier, &chord.node.Identifier)
		fmt.Printf("Offset %01x, Absolute: %01x: %s\n", mod(d), &finger.Identifier, finger.NodeAddress)
	}
}

func (chord *Chord) parseInput(input string) {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()

	args := strings.Split(input, " ")

	command := args[0]

	switch command {
	case "hash\n":
		printHash(&chord.node.Identifier)
	case "dump\n":
		chord.dump()
	case "StoreFile":
		if len(args) != 2 {
			fmt.Println("Wrong usage of StoreFile: Usage: StoreFile <localFilePath>")
			return
		}
		arg := args[1][:len(args[1])-1] //remove \n
		fileContent := getFileContent(arg)
		if fileContent == nil {
			chord.tracer.Trace("File empty: Abort StoreFile")
			return
		}
		storeFileReq := StoreFileRequest{*hashString(NodeAddress(arg)), arg, fileContent}
		chord.CallStoreFile(&storeFileReq)
	default:
		fmt.Println("Command not found: try help to see a list of commands")
	}
}

func getFileContent(filePath string) []byte {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Couldn't open file: %v\n", err)
		return nil
	}

	content, rerr := io.ReadAll(file)
	if rerr != nil {
		fmt.Printf("Couldn't read file %v\n", rerr)
		return nil
	}
	return content
}

////////////////////////////////////////////////////
//////////////// Interval functions ////////////////
////////////////////////////////////////////////////

func (chord *Chord) insertSuccessor(succNode NodeAddress) {
	chord.chordSync.Lock()
	chord.node.Successors = append([]NodeAddress{succNode}, chord.node.Successors[1:]...)
	chord.chordSync.Unlock()
}

func (chord *Chord) Create() {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	chord.tracer.Trace("Node %s: I am creating chord ring!", chord.node.NodeAddress)

	// Create ring: The own node is both successor and predecessor
	chord.node.Successors[0] = "X"
	chord.node.Predecessor = "X"
}

func (chord *Chord) FindSuccessor(findReq *FindRequest, findResp *Response) error {
	// Call succ and see if I am still its predecessor
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()

	retCode := chord.find(&findReq.QueryIdentifier, findResp)
	if retCode == PASSALONG {
		findResp.IsSuccessor = false
		// If finger table is not setup properly then you'll ask your succ
		if findResp.NodeAddress == "X" {
			chord.tracer.Trace("Pass question along to the specified node")
			findResp.NodeAddress = chord.node.Successors[0]

		}
	}

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

func (chord *Chord) StoreFile(storeFileReq *StoreFileRequest, storeFileResp *StoreFileResponse) error {
	file, err := os.Create(storeFileReq.FileName)

	if err != nil {
		fmt.Printf("Cannot create file %s: %v", storeFileReq.FileName, err)
		storeFileResp.FileWriteSuccess = err
		return err
	}
	defer file.Close()
	_, werr := file.Write(storeFileReq.FileContent)

	if werr != nil {
		fmt.Printf("Write to file %s failed: %v", storeFileReq.FileName, werr)
		storeFileResp.FileWriteSuccess = werr
		return werr
	}
	chord.tracer.Trace("Stored file %s SUCCESS", storeFileReq.FileName)
	storeFileResp.FileWriteSuccess = nil
	return nil
}

func (chord *Chord) resolveFinger(nodeToAsk NodeAddress, fingerIndex int) bool {
	if nodeToAsk == "X" {
		// Node has been cleared find next viable node
		nonClearedFingerIndex := chord.getNextValidFinger(fingerIndex)
		if nonClearedFingerIndex == -1 {
			nodeToAsk = chord.node.Predecessor
		} else {
			nodeToAsk = chord.node.FingerTable[nonClearedFingerIndex].NodeAddress
		}
		if nodeToAsk == "X" {
			chord.tracer.Trace("Wait to get notified, ask our predecessor")
			return false
		}
	}
	var finger *FingerEntry = &chord.node.FingerTable[fingerIndex]
	findReq := FindRequest{finger.Identifier, nodeToAsk}
	for resolveIter := 0; resolveIter < 32; resolveIter++ {
		if findReq.QueryNodeAddress == chord.node.NodeAddress {
			return false
		}

		resp := Response{}
		if !chord.CallFindSuccessor(&findReq, &resp) {
			// If node doesn't respond try the next one
			chord.tracer.Trace("Failure to contact finger %s: Retry next round", findReq.QueryNodeAddress)
			finger.NodeAddress = "X"
			//chord.clearDeadNode(findReq.QueryNodeAddress, false)
			return false
		}
		if resp.IsSuccessor {
			finger.NodeAddress = resp.NodeAddress
			break
		}
		// Query a better node for a better result
		findReq.QueryNodeAddress = resp.NodeAddress
	}
	return true
}

func (chord *Chord) FixFingers() {

	validSucc := chord.node.Successors[0] != "X"
	succId := hashString(chord.node.Successors[0])

	// chord.tracer.Trace("## Node %x ###", &chord.node.Identifier)
	for n := 1; n <= keySize; n++ {
		var finger *FingerEntry = &chord.node.FingerTable[n]
		var askNode NodeAddress

		// If between ourself and our succ => finger should point at our succ
		if validSucc && between(&chord.node.Identifier, &finger.Identifier, succId, true) {
			finger.NodeAddress = chord.node.Successors[0]
			continue
		}
		// If finger containes our address ask our succ to resolve, since we don't  know
		if validSucc && finger.NodeAddress == chord.node.NodeAddress {
			// Can't use finger address ask successor to do it for us
			askNode = chord.node.Successors[0]
		} else {
			// Use finger address to ask
			askNode = finger.NodeAddress
		}

		if !chord.resolveFinger(askNode, n) { // If this failes

		}
		// chord.tracer.Trace("Calling resolve with %s: resolved to %s", askNode, finger.NodeAddress)
		// chord.tracer.Trace("Finger %x (%s) is the closest successor", hashString(finger.NodeAddress), finger.NodeAddress)
	}
	// chord.tracer.Trace("########################\n")
}

func (chord *Chord) getNextValidFinger(start int) int {
	chord.tracer.Trace("Getting next valid finger %d", start)
	for n := start; n <= keySize; n++ {
		if chord.node.FingerTable[n].NodeAddress != "X" {
			return n
		}
	}
	return -1
}

func (chord *Chord) clearDeadNode(failedNode NodeAddress, isSucc bool) {
	// First contact all our successors if we have many
	if isSucc {
		chord.node.Successors[0] = "X"
	}

	failedId := keySize
	var succ NodeAddress = "X"
	for n := keySize; n > 0; n-- { //Find successor after our successor
		var finger *FingerEntry = &chord.node.FingerTable[n]
		if failedNode == finger.NodeAddress {
			failedId = n
			break
		}
		succ = finger.NodeAddress
	}
	if succ != "X" && succ != chord.node.NodeAddress {
		chord.CallNotify(succ)
	}

	for n := failedId; n > 0; n-- {
		var finger *FingerEntry = &chord.node.FingerTable[n]
		if finger.NodeAddress != failedNode {
			break
		}
		finger.NodeAddress = succ
	}
}

func (chord *Chord) CheckPred(_ *DeadCheck, _ *DeadCheck) error {
	return nil
}

func (chord *Chord) Notify(notifyReq *NotifyRequest, resp *NotifyResponse) error {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	chord.tracer.Trace("Got notified of new predecessor: %s", notifyReq.NodeAddress)

	if chord.node.Predecessor == "X" ||
		chord.node.Predecessor == notifyReq.NodeAddress { // No previous pred or already the same
		resp.Success = true
		chord.node.Predecessor = notifyReq.NodeAddress
		return nil
	}

	pred := hashString(chord.node.Predecessor)
	if between(pred, &notifyReq.Identifier, &chord.node.Identifier, true) {
		chord.tracer.Trace("Node %x joined between me: %x and pred: %x", &notifyReq.Identifier, &chord.node.Identifier, pred)
		chord.node.Predecessor = notifyReq.NodeAddress
		resp.Success = true
	} else {
		// Ask our predecessor since the new node is not in between me and my predecessor
		if !chord.CallCheckPred() {
			resp.Success = true
			chord.node.Predecessor = notifyReq.NodeAddress
			return nil
		}
		chord.tracer.Trace("Node %x IS NOT between me: %x and pred: %x => forward question to pred", &notifyReq.Identifier, &chord.node.Identifier, pred)
		resp.Success = false
		resp.NewPredAddress = chord.node.Predecessor
	}
	return nil
}

func (chord *Chord) Join(joinReq *JoinRequest, Response *Response) error {
	chord.chordSync.Lock()
	// Add joining node to ring
	fmt.Printf("Node: %s is trying to join, id: %01x\n", joinReq.NodeAddress, mod(&joinReq.Identifier))
	// TODO:query ring to get correct successor node
	// Fetch from finger table
	// If joinReq.Identifier isBetween me & ISSUCC -> return ISSUCC

	// else
	retCode := chord.find(&joinReq.Identifier, Response)

	chord.chordSync.Unlock()
	if retCode == MYSUCC {
		//Update all successors i.e. shift once
		chord.tracer.Trace("Node: %s set as our succ", joinReq.NodeAddress)
		chord.insertSuccessor(joinReq.NodeAddress)
	} else if retCode == SOLONODE {
		// If by myself then new node will be both pred an succ
		chord.insertSuccessor(joinReq.NodeAddress)
		chord.CallNotify(joinReq.NodeAddress)
	} else if retCode == IAMSUCC {
		// Insert between me and my pred
		chord.node.Predecessor = Response.NodeAddress
	} else {
		//Pass along
		chord.tracer.Trace("Checked in finger and got: %v\n", Response)
		Response.IsSuccessor = false
	}

	return nil
}

func (chord *Chord) find(identifier *big.Int, response *Response) RetCode {
	// Find closest node
	// If finger table is not initialized yet => call successors to resolve

	// Check if solo in ring => return self
	if chord.node.Successors[0] == "X" { // SOLO
		chord.tracer.Trace("Pred in solo case")
		//Solo in ring => return ourself as succ
		response.NodeAddress = chord.node.NodeAddress
		response.Identifier = chord.node.Identifier //ourself
		response.IsSuccessor = true
		return SOLONODE
	}

	// If between me and my successor => found the true successor
	currSucc := hashString(chord.node.Successors[0])
	if between(&chord.node.Identifier, identifier, currSucc, true) {
		response.NodeAddress = chord.node.Successors[0]
		response.Identifier = *currSucc
		response.IsSuccessor = true
		return MYSUCC
	}
	pred := hashString(chord.node.Predecessor)
	if between(pred, identifier, &chord.node.Identifier, true) {
		// Between me and my predecessor => I am the successor of the address
		response.NodeAddress = chord.node.NodeAddress
		response.Identifier = chord.node.Identifier
		response.IsSuccessor = true
		return IAMSUCC
	}

	return chord.closestPreceedingNode(identifier, response)

}

func (chord *Chord) closestPreceedingNode(identifier *big.Int, joinResp *Response) RetCode {
	// If table is not initialized, ask successor to resolve

	for n := keySize; n > 1; n-- {
		isBetween := between(&chord.node.FingerTable[n].Identifier, identifier, &chord.node.FingerTable[n-1].Identifier, true)
		if isBetween {
			chord.tracer.Trace("Found in between %d and %d: %s", n, n-1, chord.node.FingerTable[n-1].NodeAddress)
			joinResp.Identifier = chord.node.FingerTable[n-1].Identifier
			joinResp.NodeAddress = chord.node.FingerTable[n-1].NodeAddress
			return PASSALONG
		}
	}
	joinResp.Identifier = chord.node.FingerTable[keySize].Identifier
	joinResp.NodeAddress = chord.node.FingerTable[keySize].NodeAddress
	return PASSALONG
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
		// Test string for input
		chord.parseInput(line)
	}
}

func (chord *Chord) initClient() {
	// Default init all successors to the client self
	for n := range chord.node.Successors {
		chord.node.Successors[n] = chord.node.NodeAddress
	}

	// Default init all to client self
	chord.tracer.Trace("### Node %x ###", &chord.node.Identifier)
	for n := 1; n < len(chord.node.FingerTable); n++ {
		chord.node.FingerTable[n].Identifier = *jumpIdentifier(&chord.node.Identifier, n)
		chord.node.FingerTable[n].NodeAddress = chord.node.NodeAddress
		chord.tracer.Trace("Index %d has absolute finger: %x", n, &chord.node.FingerTable[n].Identifier)
	}
	chord.node.FingerTable[0].Identifier = chord.node.Identifier
	chord.node.FingerTable[0].NodeAddress = chord.node.NodeAddress
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
	chord.chordSync.Lock()
	stabilizeTicker := time.NewTicker(chord.intervalTimings.Stabilize)
	fingerTicker := time.NewTicker(chord.intervalTimings.FixFingers)
	checkPred := time.NewTicker(chord.intervalTimings.CheckPred)
	chord.chordSync.Unlock()
	// Creating channel using make
	tickerChan := make(chan bool)

	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case <-stabilizeTicker.C:
				// chord.chordSync.Lock()
				chord.CallStabilize()
				// chord.chordSync.Unlock()
			case <-fingerTicker.C:
				chord.chordSync.Lock()
				chord.FixFingers()
				chord.chordSync.Unlock()
			case <-checkPred.C:
				chord.chordSync.Lock()
				if !chord.CallCheckPred() {
					chord.tracer.Trace("Predecessor died: waiting to get notified")
					chord.node.Predecessor = "X"
				}
				chord.chordSync.Unlock()
			}
		}
	}()
}
