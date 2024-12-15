package main

import (
	"log"
	"math/big"
	"net"
	"net/http"
	"net/rpc"
	"strings"
)

type JoinRequest struct {
	NodeAddress NodeAddress
	Identifier  big.Int
}

type Response struct {
	IsSuccessor bool
	NodeAddress NodeAddress
	Identifier  big.Int
}

type StabilizeRequest struct {
	NodeAddress NodeAddress
	Identifier  big.Int
}

type StabilizeResponse struct {
	YouGood        bool
	NewPredAddress NodeAddress
	NodeSuccessors []NodeAddress
}

type FindRequest struct {
	QueryIdentifier  big.Int
	QueryNodeAddress NodeAddress
}

type NotifyRequest struct {
	NodeAddress NodeAddress
	Identifier  big.Int
}

type NotifyResponse struct {
	Success        bool
	NewPredAddress NodeAddress
}

type StoreFileRequest struct {
	FileIdentifier big.Int
	FileName       string
	FileContent    []byte
}

type StoreFileResponse struct {
	FileStatus FileStat
}

type RetreiveFileRequest struct {
	FileIdentifier big.Int
	FileName       string
}

type RetreiveFileResponse struct {
	NodeAddress NodeAddress
	Identifier  big.Int
	FileStatus  FileStat
	FileContent []byte
}

type DeadCheck struct {
	IsDead bool
}

func call(rpcname string, args interface{}, reply interface{}, address string) bool {
	c, err := rpc.DialHTTP("tcp", address)

	if err != nil {
		return false
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	return err == nil
}

// start a thread that listens for RPCs from worker.go
func (chord *Chord) rpcListener() {
	rpc.Register(chord)
	rpc.HandleHTTP()

	chord.tracer.Trace("Rpc handler listening on port %s", strings.Split(string(chord.node.NodeAddress), ":")[1])
	l, e := net.Listen(chord.protocol, ":"+strings.Split(string(chord.node.NodeAddress), ":")[1])
	// sockname := coordinatorSock()
	// os.Remove(sockname)
	// l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}
