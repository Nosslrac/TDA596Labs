package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/x509"
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
	fmt.Printf("### Node info ###\nNode identifier: %01x\nNode address: %s\nNode successors: %v\nNode predecessor: %v\nStored files: %v\nStored fault tolerance: %v\n\n",
		&chord.node.Identifier, chord.node.NodeAddress, chord.node.Successors, chord.node.Predecessor, chord.files, chord.replicatedFiles)
	chord.printFingers()
}

func (chord *Chord) printFingers() {
	for n := 1; n <= keySize; n++ {
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
	case "help\n":
		fmt.Printf("Commands:\n	StoreFile <localPath>\n	Lookup <fileName>\n	PrintState (alias d)\n	setpw <16 character string>")
	case "setpw":
		arg := args[1][:len(args[1])-1] //remove \n
		if len(arg) != 16 {
			fmt.Printf("Invalid password length: %d\n", len(arg))
			return
		}
		chord.encKey = []byte(arg)
	case "hash\n":
		printHash(&chord.node.Identifier)
	case "d\n":
		chord.dump()
	case "StoreFile":
		if len(args) != 2 {
			fmt.Println("Wrong usage of StoreFile: Usage: StoreFile <localFilePath>")
			return
		}
		arg := args[1][:len(args[1])-1] //remove \n
		fileContent := getFileContent(arg, chord.encKey)
		if fileContent == nil {
			chord.tracer.Trace("File empty: Abort StoreFile")
			return
		}
		file := hashString(NodeAddress(arg))
		printHash(file)
		storeFileReq := StoreFileRequest{*file, arg, fileContent, false}
		chord.chordSync.Unlock()
		chord.CallStoreFile(&storeFileReq)
		chord.chordSync.Lock()
	case "Lookup":
		if len(args) != 2 {
			fmt.Println("Wrong usage of Lookup: Usage: Lookup <fileName>")
			return
		}
		arg := args[1][:len(args[1])-1] //remove \n
		file := hashString(NodeAddress(arg))
		printHash(file)
		retreiveFileReq := RetreiveFileRequest{*file, arg}
		chord.CallLookup(&retreiveFileReq)
	case "PrintState":
		chord.dump()
	default:
		fmt.Println("Command not found: try help to see a list of commands")
	}
}

func getFileContent(filePath string, key []byte) []byte {
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

	content, err = encrypt(content, key)
	if err != nil {
		fmt.Printf("Could not encrypt content: \n %v \n", err)
		return nil
	}
	return content
}

////////////////////////////////////////////////////
//////////////// Interval functions ////////////////
////////////////////////////////////////////////////

// Call under lock
func (chord *Chord) verifySuccs(stabilizeResp *StabilizeResponse) {
	for n := 1; n < len(chord.node.Successors); n++ {
		succsSuccessor := stabilizeResp.NodeSuccessors[n-1]
		if succsSuccessor == "X" {
			return
		}
		if chord.node.Successors[n] == "X" {
			chord.node.Successors[n] = succsSuccessor
		}
		if succsSuccessor != chord.node.Successors[n] {
			// chord.tracer.Trace("Discrepancy: our succs are not coherent with our succ's succs (Us: %s Them %s)", chord.node.Successors[n], succsSuccessor)
			chord.node.Successors[n] = succsSuccessor
		}

	}
}

func (chord *Chord) Create() {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	chord.tracer.Trace("Node %s: I am creating chord ring!", chord.node.NodeAddress)

	// Create ring: The own node is both successor and predecessor
	chord.node.Successors[0] = chord.node.NodeAddress
	chord.node.Predecessor = chord.node.NodeAddress
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
		stabilizeResp.NodeSuccessors = chord.node.Successors
	} else if chord.node.Predecessor == stabilizeReq.NodeAddress {
		stabilizeResp.YouGood = true
		stabilizeResp.NodeSuccessors = chord.node.Successors
	} else {
		stabilizeResp.YouGood = false
		stabilizeResp.NewPredAddress = chord.node.Predecessor
	}
	return nil
}

func (chord *Chord) StoreFile(storeFileReq *StoreFileRequest, storeFileResp *StoreFileResponse) error {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	var filePath string
	if storeFileReq.DuplicationReq {
		filePath = fmt.Sprintf("%xReplicated/", &chord.node.Identifier) + storeFileReq.FileName
	} else {
		filePath = fmt.Sprintf("%xFiles/", &chord.node.Identifier) + storeFileReq.FileName
	}

	file, err := os.Create(filePath)

	if err != nil {
		fmt.Printf("Cannot create file %s: %v\n", filePath, err)
		storeFileResp.FileStatus = CREATEERR
		return err
	}
	defer file.Close()
	_, werr := file.Write(storeFileReq.FileContent)

	if werr != nil {
		fmt.Printf("Write to file %s failed: %v", storeFileReq.FileName, werr)
		storeFileResp.FileStatus = WRITEERR
		return werr
	}
	storeFileResp.FileStatus = FILEOK

	if storeFileReq.DuplicationReq { //Duplicate to successor if it is NOT a duplication request
		chord.tracer.Trace("File %s duplicated", storeFileReq.FileName)
		chord.replicatedFiles = append(chord.replicatedFiles, storeFileReq.FileName)
		return nil
	}

	chord.tracer.Trace("Stored file %s", storeFileReq.FileName)
	chord.files = append(chord.files, storeFileReq.FileName)
	storeFileReq.DuplicationReq = true
	chord.chordSync.Unlock()
	chord.CallDuplication(storeFileReq)
	chord.chordSync.Lock()
	return nil
}

func (chord *Chord) LookUp(retreiveFileReq *RetreiveFileRequest, retreiveFileResp *RetreiveFileResponse) error {

	filePath := fmt.Sprintf("%xFiles/%s", &chord.node.Identifier, retreiveFileReq.FileName)

	file, err := os.Open(filePath)

	if err != nil {
		retreiveFileResp.FileContent = nil
		retreiveFileResp.FileStatus = NOSUCHFILE
		return nil
	}
	defer file.Close()

	retreiveFileResp.FileContent, err = io.ReadAll(file)

	if err != nil {
		retreiveFileResp.FileContent = nil
		retreiveFileResp.FileStatus = READERR
		return nil
	}
	chord.tracer.Trace("File fetched %s SUCCESS", retreiveFileReq.FileName)
	retreiveFileResp.NodeAddress = chord.node.NodeAddress
	retreiveFileResp.Identifier = chord.node.Identifier
	retreiveFileResp.FileStatus = FILEOK
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
			return false
		}
	}

	var finger *FingerEntry = &chord.node.FingerTable[fingerIndex]
	if nodeToAsk == chord.node.NodeAddress { // Asking ourself means that we are the successor
		finger.NodeAddress = chord.node.NodeAddress
		return true
	}
	findReq := FindRequest{finger.Identifier, nodeToAsk}
	for resolveIter := 0; resolveIter < 32; resolveIter++ {
		resp := Response{}
		chord.chordSync.Unlock()
		if !chord.CallFindSuccessor(&findReq, &resp) {
			// If node doesn't respond try the next one
			// chord.tracer.Trace("Failure to contact finger %s: Retry next round", findReq.QueryNodeAddress)
			chord.chordSync.Lock()
			finger.NodeAddress = "X"
			return false
		}
		chord.chordSync.Lock()
		if resp.IsSuccessor {
			finger.NodeAddress = resp.NodeAddress
			break
		}
		// Query a better node for a better result
		findReq.QueryNodeAddress = resp.NodeAddress

		if findReq.QueryNodeAddress == chord.node.NodeAddress {
			finger.NodeAddress = chord.node.NodeAddress
			break
		}
	}
	return true
}

func (chord *Chord) FixFingers() {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	validSucc := chord.node.Successors[0] != "X"
	succId := hashString(chord.node.Successors[0])

	// chord.tracer.Trace("## Node %x ###", &chord.node.Identifier)
	chord.node.NextFinger = (chord.node.NextFinger + 1) % keySize

	var finger *FingerEntry = &chord.node.FingerTable[chord.node.NextFinger]
	var askNode NodeAddress

	// If between ourself and our succ => finger should point at our succ
	if validSucc && between(&chord.node.Identifier, &finger.Identifier, succId, true) {
		finger.NodeAddress = chord.node.Successors[0]
	}
	// If finger containes our address ask our succ to resolve, since we don't  know
	if validSucc && finger.NodeAddress == chord.node.NodeAddress {
		// Can't use finger address ask successor to do it for us
		askNode = chord.node.Successors[0]
	} else {
		// Use finger address to ask
		askNode = finger.NodeAddress
	}

	if !chord.resolveFinger(askNode, chord.node.NextFinger) { // If this failes

	}
}

// Executed under lock
func (chord *Chord) findNewSucc() {
	// TODO: read succ list instead
	if len(chord.node.Successors) > 1 {
		chord.node.Successors = append(chord.node.Successors[1:], "X")
		return
	}

	validFingerIndex := chord.getNextValidFinger(1)

	if validFingerIndex == -1 {
		if chord.node.Predecessor == "X" {
			chord.tracer.Trace("Isolated node: only hope is a notify call")
		}
		chord.node.Successors[0] = chord.node.Predecessor
		return
	}
	newSucc := chord.node.FingerTable[validFingerIndex].NodeAddress

	for n := validFingerIndex - 1; n > 0; n-- {
		chord.node.FingerTable[n].NodeAddress = newSucc
	}

}

func (chord *Chord) getNextValidFinger(start int) int {
	for n := start; n <= keySize; n++ {
		if chord.node.FingerTable[n].NodeAddress != "X" {
			return n
		}
	}
	return -1
}

func (chord *Chord) clearDeadSucc() {
	// First contact all our successors if we have many

	failedNode := chord.node.Successors[0]
	// Collapse failed node
	tmpSuccs := make([]NodeAddress, 0)
	failed := make([]NodeAddress, 0)
	for _, succ := range chord.node.Successors {
		if succ == failedNode {
			failed = append(failed, "X")
		} else {
			tmpSuccs = append(tmpSuccs, succ)
		}
	}
	chord.node.Successors = append(tmpSuccs, failed...)

	failedId := keySize
	var succ NodeAddress = chord.node.Predecessor // Ask pred to resolve new largest finger entry
	for n := keySize; n > 0; n-- {                //Find successor after our successor
		var finger *FingerEntry = &chord.node.FingerTable[n]
		if failedNode == finger.NodeAddress {
			failedId = n
			break
		}
		succ = finger.NodeAddress
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
		chord.tracer.Trace("Node %x IS NOT between me: %x and pred: %x => forward question to pred", &notifyReq.Identifier, &chord.node.Identifier, pred)
		resp.Success = false
		resp.NewPredAddress = chord.node.Predecessor
	}
	return nil
}

func (chord *Chord) Join(joinReq *JoinRequest, Response *Response) error {
	chord.chordSync.Lock()
	defer chord.chordSync.Unlock()
	// Add joining node to ring
	fmt.Printf("Node: %s is trying to join, id: %01x\n", joinReq.NodeAddress, mod(&joinReq.Identifier))
	// TODO:query ring to get correct successor node
	// Fetch from finger table
	// If joinReq.Identifier isBetween me & ISSUCC -> return ISSUCC

	// else
	retCode := chord.find(&joinReq.Identifier, Response)

	if retCode == MYSUCC {
		//Update all successors i.e. shift once
		chord.tracer.Trace("Node: %s set as our succ", joinReq.NodeAddress)
		chord.node.Successors[0] = joinReq.NodeAddress
	} else if retCode == SOLONODE {
		// If by myself then new node will be both pred an succ
		chord.node.Successors[0] = joinReq.NodeAddress
	} else if retCode == IAMSUCC {
		// Insert between me and my pred
		chord.node.Predecessor = Response.NodeAddress
	} else {
		//Pass along
		chord.tracer.Trace("Node %s should ask this node %s\n", joinReq.NodeAddress, Response.NodeAddress)
		Response.IsSuccessor = false
	}

	return nil
}

func (chord *Chord) find(identifier *big.Int, response *Response) RetCode {
	// Find closest node
	// If finger table is not initialized yet => call successors to resolve

	for i, _ := range chord.node.Successors {
		// Check if solo in ring => return self
		if chord.node.Successors[i] == "X" { // SOLO
			chord.tracer.Trace("Pred in solo case")
			//Solo in ring => return ourself as succ
			response.NodeAddress = chord.node.NodeAddress
			response.Identifier = chord.node.Identifier //ourself
			response.IsSuccessor = true
			return SOLONODE
		}

		// If between me and my successor => found the true successor
		currSucc := hashString(chord.node.Successors[i])
		if between(&chord.node.Identifier, identifier, currSucc, true) {
			response.NodeAddress = chord.node.Successors[i]
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
	}

	return chord.closestPreceedingNode(identifier, response)

}

func (chord *Chord) closestPreceedingNode(identifier *big.Int, joinResp *Response) RetCode {
	// If table is not initialized, ask successor to resolve

	for n := keySize; n >= 1; n-- {
		isBetween := between(&chord.node.Identifier, identifier, &chord.node.FingerTable[n].Identifier, true)
		if isBetween {
			// chord.tracer.Trace("Found in between %d and %d: %s", n, n-1, chord.node.FingerTable[n-1].NodeAddress)
			joinResp.Identifier = chord.node.FingerTable[n].Identifier
			joinResp.NodeAddress = chord.node.FingerTable[n].NodeAddress
			return PASSALONG
		}
	}
	joinResp.NodeAddress = chord.node.Successors[0]
	return PASSALONG
}

func main() {
	chord := getArgs()
	chord.initClient()
	chord.initDuplication()

	go chord.rpcListener()
	fmt.Printf("Chord started: node address: %v\n", chord.node.NodeAddress)
	printHash(&chord.node.Identifier)
	if chord.joinNodeIp == "XXX" {
		chord.node.JoinPending = false
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

func (chord *Chord) initDuplication() {
	duplDir := fmt.Sprintf("%xReplicated", &chord.node.Identifier)
	fileDir := fmt.Sprintf("%xFiles", &chord.node.Identifier)

	if err := os.Mkdir(duplDir, 0o700); err != nil && os.IsExist(err) {
		os.RemoveAll(duplDir)
	}
	if err := os.Mkdir(fileDir, 0o700); err != nil && os.IsExist(err) {
		os.RemoveAll(fileDir)
	}
	os.Mkdir(fileDir, 0o700)
	os.Mkdir(duplDir, 0o700)
}

func (chord *Chord) reassignToPred() {
	if len(chord.files) == 0 || chord.node.Predecessor == chord.node.NodeAddress {
		return
	}
	// Called when our predecessor dies => store all files that we replicated
	fileDir := fmt.Sprintf("%xFiles/", &chord.node.Identifier)

	var keepFiles []string

	for _, entry := range chord.files {

		fileId := hashString(NodeAddress(entry))
		if between(hashString(chord.node.Predecessor), fileId, &chord.node.Identifier, true) {
			// Send to predecessor
			keepFiles = append(keepFiles, entry)
			continue
		}

		content, err := os.ReadFile(fileDir + entry)

		if err != nil {
			log.Printf("Failure redist %s to our predecessor: %v", entry, err)
			continue
		}

		os.Remove(fileDir + entry)
		storeFileReq := StoreFileRequest{*fileId, entry, content, false}
		chord.chordSync.Unlock()
		chord.CallStoreFile(&storeFileReq)
		chord.chordSync.Lock()
	}
	if len(keepFiles) < len(chord.files) {
		chord.tracer.Trace("Moving some files to our predecessor")
		chord.files = keepFiles
	}
}

func (chord *Chord) reassignDuplicatedToMe() {
	if len(chord.replicatedFiles) == 0 {
		return
	}
	// Called when our predecessor dies => store all files that we replicated
	chord.tracer.Trace("Moving replicated files to our files")
	duplDir := fmt.Sprintf("%xReplicated", &chord.node.Identifier)
	entries, err := os.ReadDir(duplDir)
	if err != nil {
		log.Printf("Failure to move replicated files: %v", err)
	}

	fileDir := fmt.Sprintf("%xFiles/", &chord.node.Identifier)
	duplDir += "/"
	for _, entry := range entries {
		if err := os.Rename(duplDir+entry.Name(), fileDir+entry.Name()); err != nil {
			log.Printf("Failure moving %s: %v", entry.Name(), err)
		}
		file, err := os.Open(fileDir + entry.Name())
		if err != nil {
			log.Printf("Failure sending %s to our successor: %v", entry.Name(), err)
			continue
		}

		content, err := io.ReadAll(file)
		if err != nil {
			log.Printf("Failure sending %s to our successor: %v", entry.Name(), err)
			continue
		}

		storeFileReq := StoreFileRequest{*hashString(NodeAddress(entry.Name())), entry.Name(), content, true}
		chord.CallDuplication(&storeFileReq)
	}
	chord.files = append(chord.files, chord.replicatedFiles...)
	chord.replicatedFiles = chord.replicatedFiles[:0] // Clear replicated files
}

func (chord *Chord) sendFilesToPred() {

}

func getArgs() Chord {
	address := flag.String("a", "localhost", "Address of chord client")
	portvar := flag.String("p", "1234", "a port")

	joinAddress := flag.String("ja", "XXX", "Address of node in chord ring")
	joinPort := flag.String("jp", "2222", "Port of node in chord ring")

	stabilize := flag.Int("ts", 2000, "Interval between stabilize")
	fixFingers := flag.Int("tff", 2000, "Interval between fix fingers")
	checkPred := flag.Int("tcp", 2000, "Interval between check predecessors")
	encode := flag.String("e", "1623456022901234", "Encoded key")

	identifier := flag.String("i", "XXX", "Identifier")
	numSucc := flag.Int("r", 2, "Number of successors")
	verbose := flag.Bool("v", false, "a bool")
	flag.Parse()

	if !verifyRange(*stabilize, *fixFingers, *checkPred) {
		log.Fatalf("Arguments for --ts --tff --tcp not in range")
	}

	rootCAcert, err := os.ReadFile("cert/ca-cert.pem")
	if err != nil {
		log.Fatalf("Cannot load certificate: %v", err)
	}

	certPool := x509.NewCertPool()

	if !certPool.AppendCertsFromPEM(rootCAcert) {
		log.Fatal("Failed to add CAs certificate")
	}

	nodeAddress := *address + ":" + *portvar
	id := getIdentifier(NodeAddress(nodeAddress), *identifier)
	return Chord{"tcp",
		NodeInfo{*id, NodeAddress(nodeAddress), "", 0, make([]FingerEntry, keySize+1), make([]NodeAddress, *numSucc), true},
		*numSucc, *joinAddress, *joinPort,
		Timings{
			time.Millisecond * time.Duration(*stabilize),
			time.Millisecond * time.Duration(*fixFingers),
			time.Millisecond * time.Duration(*checkPred),
		},
		*certPool,
		make([]string, 0),
		make([]string, 0),
		[]byte(*encode),
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

	chord.chordSync.Lock()
	ready := chord.node.JoinPending
	chord.chordSync.Unlock()

	count := 0
	for ready && count < 30 {
		count++
		time.Sleep(time.Millisecond * 200)
		chord.chordSync.Lock()
		ready = chord.node.JoinPending
		chord.chordSync.Unlock()
	}
	chord.tracer.Trace("Setup exited")
	if count > 25 {
		chord.dump()
		log.Fatalf("Setup took too long: exiting")
	}

	go func() {
		for {
			select {
			case <-tickerChan:
				return
			// interval task
			case <-stabilizeTicker.C:
				chord.CallStabilize()
			case <-fingerTicker.C:
				chord.FixFingers()
			case <-checkPred.C:
				if !chord.CallCheckPred() {
					// chord.tracer.Trace("Predecessor died: waiting to get notified")
					chord.chordSync.Lock()
					chord.reassignDuplicatedToMe()
					chord.node.Predecessor = "X"
					chord.chordSync.Unlock()
				} else {
					chord.chordSync.Lock()
					chord.reassignToPred()
					chord.chordSync.Unlock()
				}
			}
		}
	}()
}

func encrypt(content []byte, encKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return content, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return content, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return content, err
	}

	return gcm.Seal(nonce, nonce, content, nil), nil
}

func decrypt(content []byte, encKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return content, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return content, err
	}
	nonce := content[:gcm.NonceSize()]
	content = content[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, content, nil)
	if err != nil {
		return content, err
	}
	return plainText, nil
}
