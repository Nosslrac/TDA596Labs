package mr

import (
	"fmt"
	"io"
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
	IpAddress      string
	CoordMutex     sync.Mutex
	IsDone         bool
	Nreduce        int
	mapperTimeout  []int
	reducerTimeout []int
	Workers        map[int]string //JobIds -> ipaddresses
	NextWorkerId   int

	//Private
	mapper  MapTracker
	reducer ReduceTracker
}

type WorkChunk struct {
	WorkerId  int
	Completed bool
}

type ReduceTracker struct {
	currentJob        int
	completedReducers []WorkChunk
	reduceWorkerIds   map[int]bool
}

type MapTracker struct {
	files            []string
	numFiles         int
	currentJob       int
	completedMappers []WorkChunk
	mapWorkerIds     map[int]bool
}

func (reducer *ReduceTracker) isDone() bool {
	for _, chunk := range reducer.completedReducers {
		if !chunk.Completed {
			return false
		}
	}
	return true
}

func (mapper *MapTracker) isDone() bool {
	for _, chunk := range mapper.completedMappers {
		if !chunk.Completed {
			return false
		}
	}
	return true
}

func (c *Coordinator) getMapJob(reply *WorkReply, workerId int) {
	if c.mapper.currentJob == c.mapper.numFiles {
		// Worker should wait or replace timed out mapper
		if c.retryMapper(reply, workerId) {
			// Resend other mappers work
			return
		}
		reply.WorkType = WAIT
		return
	}
	c.mapper.mapWorkerIds[workerId] = true // Worker marked as having produced mapped content

	reply.WorkType = MAP
	reply.MapFileName = c.mapper.files[c.mapper.currentJob]
	reply.MapFileContent = c.getMapFileContent(c.mapper.currentJob)
	reply.JobId = c.mapper.currentJob
	reply.NReduce = c.Nreduce
	reply.NumFiles = c.mapper.numFiles
	c.mapper.completedMappers[c.mapper.currentJob].WorkerId = workerId
	c.mapperTimeout[c.mapper.currentJob] = 1 //Start timeout timer
	c.mapper.currentJob++
}

func (c *Coordinator) getReduceJob(reply *WorkReply, workerId int) {
	// Check for time out from other reducers
	if c.reducer.currentJob == c.Nreduce {
		if c.retryReducer(reply, workerId) {
			return
		}
		reply.WorkType = WAIT
		return
	}
	c.reducer.reduceWorkerIds[workerId] = true

	reply.WorkType = REDUCE
	reply.JobId = c.reducer.currentJob
	reply.NumFiles = c.mapper.numFiles
	reply.NReduce = c.Nreduce
	reply.ReduceFileLocations = c.getReduceFileLocations()
	c.reducerTimeout[c.reducer.currentJob] = 1 // Start timeout timer
	c.reducer.completedReducers[c.reducer.currentJob].WorkerId = workerId
	c.reducer.currentJob++
}

func (c *Coordinator) GetWork(request *WorkRequest, reply *WorkReply) error {
	c.CoordMutex.Lock()
	if c.IsDone {
		reply.WorkType = NOWORK
		c.CoordMutex.Unlock()
		return nil
	}

	if c.mapper.isDone() {
		c.getReduceJob(reply, request.WorkerId)
		c.CoordMutex.Unlock()
		return nil
	}
	c.getMapJob(reply, request.WorkerId)
	c.CoordMutex.Unlock()
	return nil
}

func (c *Coordinator) WorkerDead(faildJob *JobFailed, _ *WorkReply) error {
	c.CoordMutex.Lock()
	c.clearAllWork(faildJob.UnreachableWorker.WorkerId)
	c.reducer.completedReducers[faildJob.JobId].WorkerId = 0 // Failing job will have to be restarted
	c.reducerTimeout[faildJob.JobId] = 0
	c.CoordMutex.Unlock()
	return nil
}

func (c *Coordinator) clearAllWork(workerId int) {
	fmt.Printf("Clearing work for %d\n", workerId)
	c.clearReduceDeadWorker(workerId)
	c.clearMapDeadWorker(workerId)
	delete(c.mapper.mapWorkerIds, workerId)     // Remove active workers
	delete(c.reducer.reduceWorkerIds, workerId) // Clear the job that has been done by failed worker
	delete(c.Workers, workerId)                 // Remove from worker list
}

func (c *Coordinator) DoSetup(request *SetupRequest, reply *WorkSetup) error {
	c.CoordMutex.Lock()
	reply.WorkerId = c.NextWorkerId
	c.Workers[c.NextWorkerId] = request.IPAddress
	c.NextWorkerId++
	reply.NReduce = c.Nreduce
	reply.NumFiles = c.mapper.numFiles
	c.CoordMutex.Unlock()
	return nil
}

func (c *Coordinator) WorkDone(complete *WorkComplete, reply *WorkReply) error {

	c.CoordMutex.Lock()
	if complete.WorkType == MAP {
		c.mapper.completedMappers[complete.JobId].Completed = true
	} else if complete.WorkType == REDUCE {
		fileName := fmt.Sprintf("mr-out-%d", complete.JobId)
		if file, err := os.Create("output/" + fileName); err == nil {
			fmt.Fprintf(file, "%s", complete.OutputFile)
			file.Close()
		}
		c.reducer.completedReducers[complete.JobId].Completed = true
	}
	c.CoordMutex.Unlock()
	return nil
}

func (c *Coordinator) serverHandler() {
	for {
		c.CoordMutex.Lock()
		mapperDone := c.mapper.isDone()
		reducerDone := c.reducer.isDone()
		if mapperDone && reducerDone {
			c.IsDone = true
		}

		if !mapperDone {
			c.updateMapper()
		} else {
			c.updateReducer()
		}
		c.CoordMutex.Unlock()
		time.Sleep(time.Second * 1)
	}
}

func (c *Coordinator) retryMapper(reply *WorkReply, workerId int) bool {
	for jobId := range c.mapperTimeout {
		isCleared := c.mapper.completedMappers[jobId].WorkerId == 0 && c.mapperTimeout[jobId] == 0

		if c.mapperTimeout[jobId] > TIMEOUT || isCleared {
			// Setup reply
			c.mapper.mapWorkerIds[workerId] = true // Add new worker if not present
			c.mapperTimeout[jobId] = 1             // Reset timeout
			c.mapper.completedMappers[jobId].WorkerId = workerId
			reply.WorkType = MAP
			reply.MapFileName = c.mapper.files[jobId]
			reply.MapFileContent = c.getMapFileContent(jobId)
			reply.JobId = jobId
			reply.NReduce = c.Nreduce
			reply.NumFiles = c.mapper.numFiles
			return true
		}
	}
	return false
}

func (c *Coordinator) retryReducer(reply *WorkReply, workerId int) bool {
	for jobId := range c.reducerTimeout {
		isCleared := c.reducer.completedReducers[jobId].WorkerId == 0 && c.reducerTimeout[jobId] == 0
		if c.reducerTimeout[jobId] > TIMEOUT || isCleared {
			// Setup reply
			c.reducer.reduceWorkerIds[workerId] = true // Add new worker if not present
			c.reducerTimeout[jobId] = 1
			c.reducer.completedReducers[jobId].WorkerId = workerId
			reply.ReduceFileLocations = c.getReduceFileLocations()
			reply.WorkType = REDUCE
			reply.JobId = jobId
			reply.NumFiles = c.mapper.numFiles
			reply.NReduce = c.Nreduce
			return true
		}
	}
	return false
}

func (c *Coordinator) clearMapDeadWorker(workerId int) {
	for i := range c.mapper.completedMappers {
		if c.mapper.completedMappers[i].WorkerId == workerId {
			c.mapper.completedMappers[i].WorkerId = 0      // Reset for new work
			c.mapper.completedMappers[i].Completed = false // Reset for new work
			c.mapperTimeout[i] = 0
		}
	}
}

func (c *Coordinator) clearReduceDeadWorker(workerId int) {
	for i := range c.reducer.completedReducers {
		if c.reducer.completedReducers[i].WorkerId == workerId {
			c.reducer.completedReducers[i].WorkerId = 0 // Reset for new work
			c.reducer.completedReducers[i].Completed = false
			c.reducerTimeout[i] = 0 // Reset timeout counter
		}
	}
}

// Keep track of timeouts
func (c *Coordinator) updateMapper() {
	for n := range c.mapperTimeout {
		workChunk := &c.mapper.completedMappers[n]
		inProgress := workChunk.WorkerId > 0 && !workChunk.Completed
		if c.mapperTimeout[n] > 0 && inProgress {
			c.mapperTimeout[n]++
		}
	}
}

// Keep track of timeouts
func (c *Coordinator) updateReducer() {
	for n := range c.reducerTimeout {
		workChunk := &c.reducer.completedReducers[n]
		inProgress := workChunk.WorkerId > 0 && !workChunk.Completed
		if c.reducerTimeout[n] > 0 && inProgress {
			c.reducerTimeout[n]++
		}
	}
}

func (c *Coordinator) getMapFileContent(mapFileId int) []byte {
	file, err := os.Open(c.mapper.files[mapFileId])
	if err != nil {
		log.Fatalf("Cannot open file: %v\n", err)
	}

	if content, rerr := io.ReadAll(file); rerr == nil {
		return content
	}
	return nil
}

func (c *Coordinator) getReduceFileLocations() []WorkerLocation {
	var workerAddresses []WorkerLocation //Fetch all addresses of the workers since all buckets will be spread out
	for id := range c.mapper.mapWorkerIds {
		workerAddresses = append(workerAddresses, WorkerLocation{id, c.Workers[id]})
	}
	return workerAddresses
}

func (c *Coordinator) checkStatus() {
	fmt.Printf("Timeout mappers: %v\nTimeout reducers: %v\n",
		c.mapperTimeout, c.reducerTimeout)
}

// start a thread that listens for RPCs from worker.go
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":1234")
	// sockname := coordinatorSock()
	// os.Remove(sockname)
	// l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
func (c *Coordinator) Done() bool {
	// Your code here.
	// if c.IsDone {
	// 	c.checkStatus()
	// }
	c.CoordMutex.Lock()
	retValue := c.IsDone
	c.CoordMutex.Unlock()
	return retValue
}

// Get preferred outbound ip of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	//localAddr := conn.LocalAddr().(*net.UDPAddr)
	return conn.LocalAddr().String()
}

// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	numFiles := len(files)

	ip := GetOutboundIP()

	c := Coordinator{
		ip,
		sync.Mutex{},
		false,
		nReduce,
		make([]int, numFiles),
		make([]int, nReduce),
		make(map[int]string),
		1, //Start worker ids at 1
		MapTracker{files, numFiles, 0, make([]WorkChunk, numFiles), make(map[int]bool)},
		ReduceTracker{0, make([]WorkChunk, nReduce), make(map[int]bool)},
	}

	// Your code here.
	c.server()
	go c.serverHandler() // start server handling

	fmt.Printf("Coordinator started on %v\nnReduce = %d\n", ip, nReduce)
	return &c
}
