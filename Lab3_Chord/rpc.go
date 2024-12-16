package main

import (
	"crypto/tls"
	"log"
	"math/big"
	"net"
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
	DuplicationReq bool
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

func (chord *Chord) call(rpcname string, args interface{}, reply interface{}, address string) bool {
	conf := &tls.Config{
		RootCAs:            chord.trustedCerts.Clone(),
		InsecureSkipVerify: true,
	}

	tlsConn, err := tls.Dial("tcp", address, conf)
	if err != nil {
		log.Println(err)
		return false
	}
	defer tlsConn.Close()

	client := rpc.NewClient(tlsConn)
	err = client.Call(rpcname, args, reply)

	return err == nil

}

// Start an rpcListener server on TLS connection
func (chord *Chord) rpcListener() {

	rpc.Register(chord)
	// rpc.HandleHTTP()
	cert, err := tls.LoadX509KeyPair("cert/complete-cert.pem", "cert/server-key.pem")
	if err != nil {
		log.Fatalf("Cannot get server cert: %v", err)
	}

	if len(cert.Certificate) != 2 {
		log.Fatal("server.crt should have 2 concatenated certificates: server + CA")
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
	}
	listener, err := tls.Listen("tcp", ":"+strings.Split(string(chord.node.NodeAddress), ":")[1], conf)

	if err != nil {
		log.Fatalf("rpcListener: listen error: %v", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Handling rpc connection failure: %v", err)
			break
		}
		go handleTlsConnection(conn)
	}
}

func handleTlsConnection(conn net.Conn) {
	defer conn.Close()
	rpc.ServeConn(conn)
}
