package mr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type WorkerProcess struct {
	IsSetup              bool
	IpAddress            net.IP
	WorkerId             int
	CompletedMapTasks    []bool
	CompletedReduceTasks []bool
	CoordAddress         string
	Port                 string
}

// Map functions return a slice of KeyValue.
type KeyValue struct {
	Key   string
	Value string
}

type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string, serverAddress string, port string) {
	worker := WorkerProcess{
		false,
		GetOutboundIP(),
		-1,
		[]bool{},
		[]bool{},
		serverAddress,
		port,
	}
	reply := worker.CallSetup()

	if reply.WorkerId < 0 {
		log.Fatal("Coordinator not reachable :(")
	}
	worker.WorkerId = reply.WorkerId
	worker.IsSetup = true
	worker.CompletedMapTasks = make([]bool, reply.NumFiles)
	worker.CompletedReduceTasks = make([]bool, reply.NReduce)

	go worker.worker() // Handle rpc requests
	// Get more work as long as there is work
	for {
		time.Sleep(time.Millisecond * 500)
		reply := worker.CallGetWork()
		if reply.WorkType == NOWORK {
			fmt.Println("No work left: exiting")
			return
		}
		if reply.WorkType == WAIT {
			time.Sleep(time.Second * 1) // Wait a bit then request work again
			continue
		}
		if reply.WorkType == MAP {
			//fmt.Println("Do some MAPPING", reply.JobId)
			worker.DoMap(&reply, mapf)
		} else {
			//fmt.Println("Do some REDUCING")
			worker.DoReduce(&reply, reducef)
		}
	}
}

func (worker *WorkerProcess) DoReduce(reply *WorkReply, reducef func(string, []string) string) {

	// Collect all the files into bufferedFile
	bufferedFile := []KeyValue{}

	// Get files from other people
	fileReq := FileRequest{REDUCEFILES, reply.JobId}
	for _, workerAddress := range reply.ReduceFileLocations {
		var fileReader io.Reader
		if workerAddress == worker.IpAddress.String() {
			fileReader = bytes.NewReader(worker.retreiveBucketFiles(reply.JobId))
		} else {
			mappedFiles := worker.CallGetFiles(&fileReq, workerAddress)
			fileReader = bytes.NewReader(mappedFiles.FileData)
		}

		var tmpBuffer []KeyValue

		dec := json.NewDecoder(fileReader)
		for {
			var keyValuePair KeyValue
			if err := dec.Decode(&keyValuePair); err != nil {
				break
			}
			tmpBuffer = append(tmpBuffer, keyValuePair)
		}
		bufferedFile = append(bufferedFile, tmpBuffer...)
	}

	// Sort the bucket
	sort.Sort(ByKey(bufferedFile))

	ofileName := fmt.Sprintf("mr-out-%d", reply.JobId)
	ofile, _ := os.Create(ofileName)

	length := len(bufferedFile)
	for i := 0; i < length; {
		values := []string{}
		j := 0
		// Since the bucket is sorted all keys of same kind will be consequtive
		for j = i; j < length && bufferedFile[j].Key == bufferedFile[i].Key; j++ {
			values = append(values, bufferedFile[j].Value)
		}
		output := reducef(bufferedFile[i].Key, values)
		fmt.Fprintf(ofile, "%v %v\n", bufferedFile[i].Key, output)
		i = j
	}

	worker.CallDone(reply.WorkType, ofileName, reply.JobId)
}

func (worker *WorkerProcess) DoMap(reply *WorkReply, mapf func(string, string) []KeyValue) {
	intermediate := make([][]KeyValue, reply.NReduce)

	for _, keyValuePair := range mapf(reply.MapFileName, string(reply.MapFileContent)) {
		bucketId := ihash(keyValuePair.Key) % reply.NReduce
		intermediate[bucketId] = append(intermediate[bucketId], keyValuePair)
	}

	//Create a intermediate file for each bucket for this map worker
	for n := range intermediate {
		fileName := fmt.Sprintf("mr-%d-%d", reply.JobId, n)
		file, err := os.Create(fileName)
		if err != nil {
			log.Fatalf("cannot open %v", fileName)
		}

		enc := json.NewEncoder(file)
		for _, kv := range intermediate[n] {
			if err := enc.Encode(&kv); err != nil {
				log.Fatalf("Couldn't encode %v", fileName)
			}
		}
	}
	worker.CallDone(reply.WorkType, "", reply.JobId)
}

func (worker *WorkerProcess) CallSetup() WorkSetup {
	workReq := SetupRequest{worker.IpAddress}
	workReply := WorkSetup{}
	if ok := call("Coordinator.DoSetup", &workReq, &workReply, worker.CoordAddress); ok {
		return workReply
	}
	return WorkSetup{-1, 0, 0}
}

func (worker *WorkerProcess) CallGetWork() WorkReply {
	workReq := WorkRequest{worker.WorkerId}
	workReply := WorkReply{}

	if ok := call("Coordinator.GetWork", &workReq, &workReply, worker.CoordAddress); ok {
		return workReply
	}
	return WorkReply{NOWORK, -1, -1, -1, "", nil, nil}
}

func (worker *WorkerProcess) CallDone(workType Method, outFile string, JobId int) {
	workComplete := WorkComplete{workType, JobId, outFile, worker.WorkerId}
	worker.updateWorkingCompletedFiles(workType, JobId)
	if ok := call("Coordinator.WorkDone", &workComplete, nil, worker.CoordAddress); !ok {
		fmt.Printf("Call failed: Coordinator not responding\n")
	}
}

func (worker *WorkerProcess) CallGetFiles(request *FileRequest, address string) MappedFiles {
	mappedFiles := MappedFiles{}
	if ok := call("WorkerProcess.GetMappedBuckets", request, &mappedFiles, address); !ok {
		fmt.Printf("Call failed: WorkerProcess not responding\n")
	}
	return mappedFiles
}

func (worker *WorkerProcess) getFiles(request *FileRequest, mappedFiles *MappedFiles) {
	if request.FileType == MAPFILES {
		mappedFiles.FileData = worker.retreiveBucketFiles(request.FileID)
	}
}

func (worker *WorkerProcess) retreiveBucketFiles(bucketId int) []byte {
	fileNamePattern := fmt.Sprintf("mr-*-%d", bucketId) // Get all files with bucket FileID

	matches, filepathError := filepath.Glob(fileNamePattern)
	if filepathError != nil {
		log.Fatalf("Malformed filepattern: %v", filepathError)
	}

	var intermediate []byte

	for _, fileName := range matches {
		file, err := os.Open(fileName)
		if err != nil {
			log.Fatalf("File couldn't be opened: %v", err)
		}
		content, _ := io.ReadAll(file)
		intermediate = append(intermediate, content...)
	}
	return intermediate
}

func (worker *WorkerProcess) updateWorkingCompletedFiles(workType Method, JobId int) {
	if workType == MAP {
		worker.CompletedMapTasks[JobId] = true
	} else {
		worker.CompletedReduceTasks[JobId] = true
	}
}

func (worker *WorkerProcess) GetMappedBuckets(request *FileRequest, mappedFiles *MappedFiles) error {
	worker.getFiles(request, mappedFiles)
	return nil
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}, address string) bool {
	c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	// sockname := coordinatorSock()
	// c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		// log.Fatal("dialing:", err)
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

// start a thread that listens for RPCs from worker.go
func (worker *WorkerProcess) worker() {
	rpc.Register(worker)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":"+worker.Port)
	// sockname := workerSock()
	// os.Remove(sockname)
	// l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}
