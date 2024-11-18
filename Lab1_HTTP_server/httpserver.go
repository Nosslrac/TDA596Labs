package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

const MAX_CONNECTIONS = 10

func getArgs() HTTPserver {
	portvar := flag.String("port", "1234", "a port")
	verbose := flag.Bool("v", false, "a bool")
	flag.Parse()
	return HTTPserver{*portvar, "tcp", 0, 0, HTTPTracer{*verbose}, *sync.NewCond(&sync.Mutex{})}
}

func (server *HTTPserver) decrement() {
	server.serverCondition.L.Lock()
	server.numConnections--
	if server.numConnections == MAX_CONNECTIONS-1 {
		server.serverCondition.Signal()
	}
	defer server.serverCondition.L.Unlock()
}

func (server *HTTPserver) connectionHandler(con net.Conn) {

	defer con.Close()
	con.SetDeadline(time.Now().Add(time.Second * 5))
	reader := bufio.NewReader(con)
	request, rerr := http.ReadRequest(reader)

	if rerr != nil {
		// Benchmarking tools might make extra connections without
		// intent on transfering data
		server.tracer.Trace("Client closed connection without sending data: %s", rerr.Error())
		server.decrement()
		return
	}

	//Init response
	response := defaultResponse(request)
	request.Response = &response

	// Handle the request: fills the request.Response with info
	server.handleHTTPRequest(request)

	//Write back on connection
	var writeBuffer bytes.Buffer
	if werr := request.Response.Write(&writeBuffer); werr != nil {
		log.Print("Write failed: ", werr)
	}
	if body := request.Response.Body; body != nil {
		body.Close()
	}
	if _, err := con.Write(writeBuffer.Bytes()); err != nil {
		log.Print("Write failed", err)
	}

	server.decrement()
}

func defaultResponse(request *http.Request) http.Response {
	return http.Response{ // Default response might change later
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Status:     "200 OK",
		StatusCode: 200,
		Close:      true,
		Header:     make(http.Header),
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

func (server *HTTPserver) handleHTTPRequest(httpReq *http.Request) {
	switch httpReq.Method {
	case "GET":
		performGet(httpReq)
	case "POST":
		performPost(httpReq)
		server.tracer.Trace("File created: %s", httpReq.RequestURI)
	default:
		httpReq.Response.Status = "501 Not Implemented"
		httpReq.Response.StatusCode = 501
		server.tracer.Trace("Received non implemented request: %s", httpReq.Method)
	}
}

func performGet(httpReq *http.Request) {
	//fmt.Println(path.Ext(httpReq.RequestURI))
	switch path.Ext(httpReq.RequestURI) {
	case ".html":
		httpReq.Response.Header.Add("Content-Type", "text/html")
	case ".txt":
		httpReq.Response.Header.Add("Content-Type", "text/plain")
	case ".gif":
		httpReq.Response.Header.Add("Content-Type", "image/gif")
	case ".jpeg":
		httpReq.Response.Header.Add("Content-Type", "image/jpeg")
	case ".jpg":
		httpReq.Response.Header.Add("Content-Type", "image/jpeg")
	case ".css":
		httpReq.Response.Header.Add("Content-Type", "text/css")
	default:
		httpReq.Response.Status = "400 Bad Request"
		httpReq.Response.StatusCode = 400
		return
	}
	workingDir, err := os.Getwd()
	if err != nil {
		erresponse := internalErrorResponse()
		httpReq.Response = &erresponse
		fmt.Println("Filesystem not readable: ", err)
		return
	}

	// Try to open the file
	// TODO: should file be closed? Concurrent requests might suffer, lock around file opening=
	file, fileError := os.Open(workingDir + httpReq.RequestURI)

	if fileError != nil {
		httpReq.Response.Status = "404 Not Found"
		httpReq.Response.StatusCode = 404
		httpReq.Response.Header.Add("Content-Length", "0")
		return
	}

	fileInfo, infoError := file.Stat()

	if infoError != nil {
		erresponse := internalErrorResponse()
		httpReq.Response = &erresponse
		fmt.Println("File not found: ", infoError)
		return
	}
	httpReq.Response.Header.Add("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	httpReq.Response.Header.Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.Name()))
	httpReq.Response.Body = file
}

func performPost(httpReq *http.Request) {
	workingDir, werr := os.Getwd()
	if werr != nil {
		erresponse := internalErrorResponse()
		httpReq.Response = &erresponse
		fmt.Println("Could not get working directory", werr)
		return
	}
	// TODO: if file exists should it be overwritten?
	outFile, cerr := os.Create(workingDir + httpReq.RequestURI)
	if cerr != nil {
		erresponse := internalErrorResponse()
		httpReq.Response = &erresponse
		fmt.Println("Could not create file", cerr)
		return
	}
	defer outFile.Close()

	if _, cperr := io.Copy(outFile, httpReq.Body); cperr != nil {
		erresponse := internalErrorResponse()
		httpReq.Response = &erresponse
		fmt.Println("Could not copy POST request to file", cperr)
		return
	}
	httpReq.Body.Close()
}

func (tracer HTTPTracer) Trace(format string, a ...any) {
	if tracer.verbose {
		fmt.Printf(format+"\n", a...)
	}
}

func main() {
	server := getArgs()
	fmt.Printf("Server started: listening on port: %s\n", server.port)

	listen, er := net.Listen(server.serverType, ":"+server.port)
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
		server.serverCondition.L.Lock()
		server.numTotal++
		for server.numConnections == MAX_CONNECTIONS {
			server.serverCondition.Wait()
			server.tracer.Trace("Server woken up, accepting new connection\n")
		}
		server.numConnections++
		if server.numConnections > MAX_CONNECTIONS {
			log.Fatal("Server capacity exceeded: EXPLODE")
		}

		server.tracer.Trace("Concurrency level %d\n", server.numConnections)

		server.serverCondition.L.Unlock()

		go server.connectionHandler(connection)

		server.tracer.Trace("Total connections served: %d", server.numTotal)

	}

}
