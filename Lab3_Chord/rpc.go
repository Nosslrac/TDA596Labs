package main

import (
	"fmt"
	"net/rpc"
)

type JoinRequest struct {
	IpAddressAndPort string
	Identifier       string
}

type JoinResponse struct {
	SuccessorNode           string
	SuccessorNodeIdentifier string
}

func call(rpcname string, args interface{}, reply interface{}, address string) bool {
	c, err := rpc.DialHTTP("tcp", address)

	if err != nil {
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
