package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

const TIMEOUT = 10

type Coordinator struct {
	// Your definitions here.
	// Public
	CoordMutex     sync.Mutex
	IsDone         bool
	Nreduce        int
	mapperTimeout  []int
	reducerTimeout []int

	//Private
	mapper  MapTracker
	reducer ReduceTracker
}

type ReduceTracker struct {
	currentJob        int
	completedReducers []bool
}

type MapTracker struct {
	files            []string
	numFiles         int
	currentJob       int
	completedMappers []bool
}

// Need to be done in critical section
func (reducer *ReduceTracker) reducerComplete(reducerId int) {
	reducer.completedReducers[reducerId] = true
}

func (reducer *ReduceTracker) isDone() bool {
	for _, val := range reducer.completedReducers {
		if !val {
			return false
		}
	}
	return true
}

// Need to be done in critical section
func (mapper *MapTracker) mapperComplete(mapperId int) {
	mapper.completedMappers[mapperId] = true
}

func (mapper *MapTracker) isDone() bool {
	for _, val := range mapper.completedMappers {
		if !val {
			return false
		}
	}
	return true
}

func (c *Coordinator) getMapJob(reply *WorkReply) {
	if c.mapper.currentJob == c.mapper.numFiles {
		// Worker should wait or replace timed out mapper
		// if c.retryMapper(reply) {
		// 	// Resend other mappers work
		// 	return
		// }
		reply.WorkType = WAIT
		return
	}
	reply.WorkType = MAP
	reply.MapFile = c.mapper.files[c.mapper.currentJob]
	reply.WorkId = c.mapper.currentJob
	reply.NReduce = c.Nreduce
	reply.NumFiles = c.mapper.numFiles
	c.mapper.currentJob++
	//fmt.Print("Reply to map: ", reply)
}

func (c *Coordinator) getReduceJob(reply *WorkReply) {
	// Check for time out from other reducers
	if c.reducer.currentJob == c.Nreduce {
		// if c.retryReducer(reply) {
		// 	return
		// }
		reply.WorkType = WAIT
		return
	}
	reply.WorkType = REDUCE
	reply.WorkId = c.reducer.currentJob
	reply.NumFiles = c.mapper.numFiles
	reply.NReduce = c.Nreduce
	c.reducer.currentJob++
	//log.Println("Reduce job already allocated, WAIT for failures")
}

func (c *Coordinator) getNextJob(reply *WorkReply) {
	c.CoordMutex.Lock()
	if c.mapper.isDone() && c.reducer.isDone() {
		//fmt.Println("Doing map")
		reply.WorkType = NOWORK
		c.CoordMutex.Unlock()
		return
	}

	if c.mapper.isDone() {
		//fmt.Println("Doing reduce")
		c.getReduceJob(reply)
		c.CoordMutex.Unlock()
		return
	}
	//fmt.Println("Doing map")
	c.getMapJob(reply)
	c.CoordMutex.Unlock()
}

func (c *Coordinator) GetWork(request *WorkRequest, reply *WorkReply) error {
	c.getNextJob(reply)
	return nil
}

func (c *Coordinator) WorkDone(complete *WorkComplete, reply *WorkReply) error {
	//log.Printf("Work done file %v, Method: %v\n", complete.OutputFile, complete.WorkType)
	c.CoordMutex.Lock()
	if complete.WorkType == MAP {
		c.mapper.mapperComplete(complete.WorkId) //Work id needed later maybe
	} else if complete.WorkType == REDUCE {
		c.reducer.reducerComplete(complete.WorkId)
	}
	c.CoordMutex.Unlock()
	return nil
}

func (c *Coordinator) serverHandler() {
	//i := 0
	for {
		c.CoordMutex.Lock()
		mapperDone := c.mapper.isDone()
		reducerDone := c.mapper.isDone()
		if mapperDone && reducerDone {
			c.IsDone = true
		}

		// if !mapperDone {
		// 	c.updateMapper()
		// } else {
		// 	c.updateReducer()
		// }

		// i = (i + 1) % 3
		// if i == 0 {
		// 	c.checkStatus()
		// }
		c.CoordMutex.Unlock()
		time.Sleep(time.Second * 1)
	}
}

func (c *Coordinator) retryMapper(reply *WorkReply) bool {
	for mapperId := range c.mapperTimeout {
		if c.mapperTimeout[mapperId] >= TIMEOUT {
			c.mapperTimeout[mapperId] = 0
			reply.WorkType = MAP
			reply.MapFile = c.mapper.files[mapperId]
			reply.WorkId = mapperId
			reply.NReduce = c.Nreduce
			reply.NumFiles = c.mapper.numFiles
			return true
		}
	}
	return false
}

func (c *Coordinator) retryReducer(reply *WorkReply) bool {
	for reducerId := range c.reducerTimeout {
		if c.reducerTimeout[reducerId] >= TIMEOUT {
			c.reducerTimeout[reducerId] = 0
			reply.WorkType = REDUCE
			reply.WorkId = reducerId
			reply.NumFiles = c.mapper.numFiles
			reply.NReduce = c.Nreduce
			return true
		}
	}
	return false
}

// Keep track of timeouts
func (c *Coordinator) updateMapper() {
	for n := range c.mapperTimeout {
		if !c.mapper.completedMappers[n] {
			c.mapperTimeout[n]++
		}
	}
}

// Keep track of timeouts
func (c *Coordinator) updateReducer() {
	for n := range c.reducerTimeout {
		if !c.reducer.completedReducers[n] {
			c.reducerTimeout[n]++
		}
	}
}

func (c *Coordinator) checkStatus() {
	fmt.Printf("Progress status: %v\n---Mappers: %v\n---Reducers: %v\n", c.IsDone, c.mapper.completedMappers, c.reducer.completedReducers)
}

// start a thread that listens for RPCs from worker.go
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

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	// Your code here.
	return c.IsDone
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	numFiles := len(files)

	//log.Printf("Coordinator setup: files %v, nReduce %d, numFiles: %d\n", files, nReduce, numFiles)
	c := Coordinator{
		sync.Mutex{},
		false,
		nReduce,
		make([]int, numFiles),
		make([]int, nReduce),
		MapTracker{files, numFiles, 0, make([]bool, numFiles)},
		ReduceTracker{0, make([]bool, nReduce)},
	}

	// Your code here.
	c.server()
	go c.serverHandler() // start server handling
	return &c
}
