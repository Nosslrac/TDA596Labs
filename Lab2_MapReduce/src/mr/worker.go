package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

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
	reducef func(string, []string) string) {

	// Get more work as long as there is work
	for {
		reply := CallGetWork()
		if reply.WorkType == NOWORK {
			//fmt.Println("No work to do: exiting")
			return
		}
		if reply.WorkType == WAIT {
			time.Sleep(time.Second * 1) // Wait a bit then request work again
			continue
		}
		if reply.WorkType == MAP {
			//fmt.Println("Do some MAPPING", reply.WorkId)
			DoMap(&reply, mapf)
		} else {
			//fmt.Println("Do some REDUCING")
			DoReduce(&reply, reducef)
		}
	}
}

func DoReduce(reply *WorkReply, reducef func(string, []string) string) {

	// Collect all the files into bufferedFile
	bufferedFile := []KeyValue{}
	for x := 0; x < reply.NumFiles; x++ {
		fileName := fmt.Sprintf("mr-%d-%d", x, reply.WorkId)
		file, _ := os.Open(fileName)

		var tmpBuffer []KeyValue

		dec := json.NewDecoder(file)
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

	ofileName := fmt.Sprintf("mr-out-%d", reply.WorkId)
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

	CallDone(reply.WorkType, ofileName, reply.WorkId)
}

func DoMap(reply *WorkReply, mapf func(string, string) []KeyValue) {
	intermediate := make([][]KeyValue, reply.NReduce)

	file, err := os.Open(reply.MapFile)
	if err != nil {
		log.Fatalf("cannot open %v", reply.MapFile)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", reply.MapFile)
	}
	file.Close()

	for _, keyValuePair := range mapf(reply.MapFile, string(content)) {
		bucketId := ihash(keyValuePair.Key) % reply.NReduce
		intermediate[bucketId] = append(intermediate[bucketId], keyValuePair)
	}

	//Create a intermediate file for each bucket for this map worker
	for n := range intermediate {
		fileName := fmt.Sprintf("mr-%d-%d", reply.WorkId, n)
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
	CallDone(reply.WorkType, "", reply.WorkId)
}

func CallGetWork() WorkReply {
	workReq := WorkRequest{true}
	workReply := WorkReply{}

	if ok := call("Coordinator.GetWork", &workReq, &workReply); ok {
		return workReply
	}
	return WorkReply{NOWORK, -1, -1, -1, ""}
}

func CallDone(workType Method, outFile string, workId int) {
	workComplete := WorkComplete{workType, workId, outFile}
	if ok := call("Coordinator.WorkDone", &workComplete, nil); !ok {
		fmt.Printf("Call failed: Coordinator not responding\n")
	}
}

// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
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
