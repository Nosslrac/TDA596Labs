package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
)


type Coordinator struct {
	// Your definitions here.
	// Public
	IsDone bool
	mapProgress []int8
	reduceProgress []int8

	//Private
	mapper MapTracker
	reducer ReduceTracker

	
}

type ReduceTracker struct {
	files []string
	currentJob int
	filesPerJob int
	jobsWithOneExtra int
	totFiles int
	WorkCounter int
}

type MapTracker struct {
	files []string
	currentJob int
	filesPerJob int
	jobsWithOneExtra int
	totFiles int
	WorkCounter int
}


func (reducer *ReduceTracker) isDone() bool {
	return reducer.currentJob == reducer.totFiles
}

func (reducer *ReduceTracker) addFile(file string) {
	reducer.files[reducer.totFiles] = file
	reducer.totFiles++
}

func (reducer *ReduceTracker) getReduceJob(reply *WorkReply) {
	if reducer.currentJob < reducer.totFiles {
		reply.Files = append(reply.Files, reducer.files[reducer.currentJob])
	}
	
	reply.WorkerId = reducer.WorkCounter
	reducer.WorkCounter++
}

func (mapper *MapTracker) isDone() bool {
	return mapper.currentJob == mapper.totFiles
}

func (mapper *MapTracker) getMapJob(reply *WorkReply) {
	start := mapper.currentJob
	end := mapper.currentJob + mapper.filesPerJob
	if mapper.jobsWithOneExtra > 0 {
		mapper.jobsWithOneExtra--
		end++
	}
	// Update Coordinator
	mapper.currentJob = end //End is not included in range

	if end > mapper.totFiles {
		log.Fatal("Coordinator corrupted: list out of bounds")
	}
	
	reply.WorkType = MAP
	reply.Files = mapper.files[start : end]
	reply.WorkerId = mapper.WorkCounter
	mapper.WorkCounter++
}


func (c *Coordinator) getNextJob(reply *WorkReply) {
	if c.mapper.isDone() && c.reducer.isDone() {
		reply.WorkType = NOWORK
		return
	}

	if c.mapper.isDone() {
		c.reducer.getReduceJob(reply)
		return
	}
	c.mapper.getMapJob(reply)
}



func (c *Coordinator) GetWork(request *WorkRequest, reply *WorkReply) error {
	log.Print("Worker requesting work, let us give them some work")
	c.getNextJob(reply)
	return nil
}

func (c *Coordinator) WorkDone(complete *WorkComplete, reply *WorkReply) error {
	log.Printf("Work done file %v, Method: %v\n", complete.OutputFile, complete.WorkType)
	if complete.WorkType == MAP {
		c.mapProgress[reply.WorkerId] = 1
		c.reducer.addFile(complete.OutputFile)
	} else if complete.WorkType == REDUCE {
		c.reduceProgress[reply.WorkerId] = 1
	}
	
	
	return nil
}


// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	// Your code here.
	return c.IsDone
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	numFiles := len(files)
	workersWithOneExtra := numFiles % nReduce
	filesPerWorker := numFiles / nReduce
	workerFiles := numFiles

	log.Printf("Coordinator setup: files %v, nReduce %d, numFiles: %d, %d, %d\n", files, nReduce, numFiles, workersWithOneExtra, filesPerWorker)
	c := Coordinator{
		false,
		make([]int8, workerFiles),
		make([]int8, workerFiles),
		MapTracker{files, 0, filesPerWorker, workersWithOneExtra, numFiles, 1},
		ReduceTracker{make([]string, workerFiles), 0, filesPerWorker, workersWithOneExtra, 0, 1},
	}

	// Your code here.
	c.server()
	return &c
}
