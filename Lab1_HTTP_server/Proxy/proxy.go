package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
)

const MAX_CONNECTIONS = 10

func (proxy *HTTPProxy) connectionHandler(clientCon net.Conn) {
	defer clientCon.Close()

	proxy.serverCondition.L.Lock()
	if proxy.numConnections > MAX_CONNECTIONS {
		log.Fatal("Over max capacity: Exiting")
	}
	proxy.serverCondition.L.Unlock()
	reader := bufio.NewReader(clientCon)
	request, rerr := http.ReadRequest(reader)
	if rerr != nil {
		// Benchmarking tools might make extra connections without
		// intent on transfering data
		response := internalErrorResponse()
		writeResponse(&response, clientCon)
		proxy.tracer.Trace("Client closed connection without sending data: %s", rerr.Error())
		return
	}
	// Only GET requests should be handled
	if !validGETRequest(request) {
		response := notImplemented(request)
		writeResponse(&response, clientCon)
		return
	}

	// Check if we need to dial the server
	// TODO check for local data first

	// Request data from the server and forward response to client
	proxy.requestAndForward(request, clientCon)

	proxy.serverCondition.L.Lock()
	proxy.numConnections--
	if proxy.numConnections == MAX_CONNECTIONS-1 {
		proxy.serverCondition.Signal()
	}
	proxy.serverCondition.L.Unlock()
}

func validGETRequest(request *http.Request) bool {
	return request.Method == "GET"
}

func (proxy *HTTPProxy) requestAndForward(request *http.Request, clientCon net.Conn) {
	defer clientCon.Close()

	if request.Host == "" {
		log.Print("HTTP request incomplete: host missing")
		response := badRequest(request)
		writeResponse(&response, clientCon)
		return
	}

	if request.Host == clientCon.LocalAddr().String() || 
	request.Host == "localhost:"+proxy.port {
		log.Print("HTTP request incomplete: proxy is target")
		response := badRequest(request)
		writeResponse(&response, clientCon)
		return
	}

	proxy.tracer.Trace("Connecting to %s server on address: %s", proxy.serverType, request.Host)
	serverCon, derr := net.Dial(proxy.serverType, request.Host)

	if derr != nil {
		// Send 404: server address not found
		log.Printf("Cannot connect to server: %s\n	Error: %s\n", request.Host, derr.Error())
		response := notFound(request)
		writeResponse(&response, clientCon)
		return
	}
	defer serverCon.Close()

	// Write request to server
	if werr := writeRequest(request, serverCon); werr != nil {
		return
	}

	// Read response
	reader := bufio.NewReader(serverCon)
	response, rerr := http.ReadResponse(reader, nil)
	if rerr != nil {
		log.Print("Read from server failed: ", rerr)
		response := badRequest(request)
		writeResponse(&response, clientCon)
		return
	}

	// Write back to client
	if werr := writeResponse(response, clientCon); werr != nil {
		log.Print("Write to client failed: ", werr)
	}
}

///////////////////////////////////////////////////////
////////////// Basic responses ////////////////////////
///////////////////////////////////////////////////////

func badRequest(request *http.Request) http.Response {
	return http.Response{
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Status:     "400 Bad Request",
		StatusCode: 400,
	}
}

func internalErrorResponse() http.Response {
	return http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Status:     "500 Internal Server Error",
		StatusCode: 500,
		Close:      true,
		Header:     make(http.Header),
	}
}

func notFound(request *http.Request) http.Response {
	return http.Response{
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Status:     "404 Not Found",
		StatusCode: 404,
	}
}

func notImplemented(request *http.Request) http.Response {
	return http.Response{
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Status:     "501 Not Implemented",
		StatusCode: 501,
	}
}

///////////////////////////////////////////////////////
/////// Writing requests and responses ////////////////
///////////////////////////////////////////////////////

func writeResponse(response *http.Response, con net.Conn) error {
	//Write to buffer
	var writeBuffer bytes.Buffer
	if werr := response.Write(&writeBuffer); werr != nil {
		log.Print("Response write to buffer failed: ", werr)
		return werr
	}
	// Write on connection
	if _, werr := con.Write(writeBuffer.Bytes()); werr != nil {
		log.Print("Buffer write on connection failed: ", werr)
		return werr
	}
	return nil
}

func writeRequest(request *http.Request, con net.Conn) error {
	//Write to buffer
	var writeBuffer bytes.Buffer
	if werr := request.Write(&writeBuffer); werr != nil {
		log.Print("Response write to buffer failed: ", werr)
		return werr
	}
	// Write on connection
	if _, werr := con.Write(writeBuffer.Bytes()); werr != nil {
		log.Print("Buffer write on connection failed: ", werr)
		return werr
	}
	return nil
}

func getArgs() HTTPProxy {
	portvar := flag.String("port", "1234", "a port")
	verbose := flag.Bool("v", false, "a bool")
	flag.Parse()
	return HTTPProxy{*portvar, "tcp", 0, 0, HTTPTracer{*verbose}, *sync.NewCond(&sync.Mutex{})}
}

func main() {
	proxy := getArgs()
	fmt.Printf("Proxy started: listening on port: %s\n", proxy.port)

	listen, er := net.Listen(proxy.serverType, ":"+proxy.port)

	if er != nil {
		log.Fatal("Couldn't open port", er)
	}
	defer listen.Close() //Close when l is handled

	for {
		connection, err := listen.Accept()
		if err != nil {
			fmt.Println("Connection failed", err)
			continue
		}
		proxy.serverCondition.L.Lock()
		proxy.numTotal++
		for proxy.numConnections == MAX_CONNECTIONS {
			proxy.serverCondition.Wait()
			//fmt.Println("Woken up")
		}
		proxy.numConnections++
		if proxy.numConnections > MAX_CONNECTIONS {
			log.Fatal("Proxy capacity exceeded: EXPLODE")
		}

		proxy.tracer.Trace("Concurrency level: %d | Total connections served: %d\n", proxy.numConnections, proxy.numTotal)

		proxy.serverCondition.L.Unlock()

		go proxy.connectionHandler(connection)
	}

}
