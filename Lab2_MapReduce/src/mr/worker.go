package mr

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// Only interested in the key, Map generates values of 1
//
type ParseLine struct {
    Key           string          `json:"Key"`
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	// uncomment to send the Example RPC to the coordinator.
	for {
		reply := CallGetWork()
		var fileName string
		if reply.WorkType == NOWORK {
			fmt.Println("No work to do: returning")
			return
		}
		if reply.WorkType == MAP {
			//fmt.Println("Do some MAPPING")
			fileName = DoMap(&reply, mapf)
		} else {
			//fmt.Println("Do some REDUCING")
			fileName = DoReduce(&reply, reducef)
		}
		CallDone(reply.WorkType, fileName)
	}
}

func DoReduce(reply *WorkReply, reducef func(string, []string) string) string {
	log.Print("Filename: ", reply.Files[0])
	file, _ := os.Open(reply.Files[0])

	occurences := make(map[string]int)
	var keyValuePair ParseLine

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if err := json.Unmarshal([]byte(scanner.Text()), &keyValuePair); err != nil {
			log.Fatalf("Object poorly formatted: %v: %v", scanner.Text(), err)
		}
		occurences[keyValuePair.Key]++
	}
	keys := make([]string, len(occurences))
	index := 0
	for key := range occurences {
		keys[index] = key
		index++
	}
	sort.Strings(keys)

	fileName := "mr-out-" + strconv.Itoa(reply.WorkerId)
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatalf("Cannot create file %v: %v", fileName, err)
	}
	defer file.Close()
	
	for _, key := range keys {
		fmt.Fprintf(file, "%v %v\n", key, occurences[key])
	}
	return fileName
}


func DoMap(reply *WorkReply, mapf func(string, string)[]KeyValue) string {
	intermediate := []KeyValue{}
	for _, filename := range reply.Files {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", filename)
		}
		file.Close()
		kva := mapf(filename, string(content))
		intermediate = append(intermediate, kva...)
	}
	workId := strconv.Itoa(reply.WorkerId)
	fileName := "mr-" +  workId + "-" + workId
	file, err := os.Create(fileName)

	if err != nil {
		log.Fatalf("cannot open %v", fileName)
	}

	enc := json.NewEncoder(file)
	for _, kv := range intermediate{
	  if err := enc.Encode(&kv); err != nil {
		log.Fatalf("Couldn't encode %v", fileName)
	  }
	}
	return fileName
}


func CallGetWork() WorkReply {
	workReq := WorkRequest{true}
	workReply := WorkReply{}

	
	if ok := call("Coordinator.GetWork", &workReq, &workReply); ok {
		// reply.Y should be 100.
		//fmt.Printf("Method: %v, Files: %v\n", workReply.WorkType, workReply.Files)
		return workReply
	} 
	fmt.Printf("call failed!\n")
	return WorkReply{NOWORK, nil, -1}
}


func CallDone(workType Method, outFile string) {
	workComplete := WorkComplete{workType, outFile}
	if ok := call("Coordinator.WorkDone", &workComplete, nil); ok {
		fmt.Printf("Work %d done\n", workComplete.WorkType)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
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

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
