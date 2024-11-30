package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

// example to show how to declare the arguments
// and reply for an RPC

type WorkerStatus bool

const (
	WORKERALIVE WorkerStatus = false
	WORKERDEAD  WorkerStatus = true
)

type Method int

const (
	MAP    Method = 1 //Do map
	REDUCE Method = 2 //Do reduce
	WAIT   Method = 3 //Standby, might be more work later
	NOWORK Method = 5 //Quit no more work
)

type FileType int

const (
	MAPFILES     FileType = 1
	REDUCEFILES  FileType = 2
	FILENOTFOUND FileType = 3
)

type WorkRequest struct {
	WorkerId int
}

type SetupRequest struct {
	IPAddress string
}

type WorkComplete struct {
	WorkType   Method
	JobId      int
	OutputFile []byte
	WorkerId   int
}

type WorkSetup struct {
	WorkerId int
	NumFiles int
	NReduce  int
}

type WorkReply struct {
	WorkType Method // MAP, REDUCE, WAIT, NOWORK
	JobId    int
	NumFiles int
	NReduce  int

	MapFileName    string
	MapFileContent []byte

	ReduceFileLocations []WorkerLocation
}

type WorkerLocation struct {
	WorkerId int
	Address  string
}

type JobFailed struct {
	UnreachableWorker WorkerLocation
	WorkType          Method
	JobId             int
}

type MappedFiles struct {
	WorkerStat WorkerStatus
	FileData   []byte
}

type FileRequest struct {
	FileType FileType
	FileID   int
}

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/5840-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}

func workerSock() string {
	s := "/var/tmp/1234-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
