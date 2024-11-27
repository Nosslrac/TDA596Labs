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
// and reply for an RPC.
type Method int

const (
	MAP    Method = 1 //Do map
	REDUCE Method = 2 //Do reduce
	WAIT   Method = 3 //Standby, might be more work later
	NOWORK Method = 4 //Quit no more work
)

type WorkRequest struct {
	Ready bool
}

type WorkComplete struct {
	WorkType   Method
	WorkId     int
	OutputFile string
}

type WorkReply struct {
	WorkType Method // MAP, REDUCE, WAIT, NOWORK
	WorkId   int
	NumFiles int
	NReduce  int

	MapFile string
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
