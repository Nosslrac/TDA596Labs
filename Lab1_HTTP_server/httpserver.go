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
)

const MAX_CONNECTIONS = 10

func getArgs() HTTPserver {
	portvar := flag.String("port", "1234", "a port")
	verbose := flag.Bool("v", false, "a bool")
	flag.Parse()
	return HTTPserver{*portvar, "tcp", 0, 0, HTTPTracer{*verbose}, *sync.NewCond(&sync.Mutex{})}
}

func connectionHandler(con net.Conn, server *HTTPserver) {
	server.serverCondition.L.Lock()
	if server.numConnections > MAX_CONNECTIONS {
		log.Fatal("Over max capacity: Exiting")
	}
	server.serverCondition.L.Unlock()
	defer con.Close()
	readBuffer := make([]byte, 1024)
	n, err := con.Read(readBuffer)
	if err != nil {
		// Benchmarking tools might make extra connections without
		// intent on transfering data
		server.tracer.Trace("Client closed connection without sending data: %s", err.Error())
		return
	}
	request := getHTTPRequest(readBuffer[:n])
	response := http.Response{ // Default response might change later
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Status:     "200 OK",
		StatusCode: 200,
		Header:     make(http.Header),
	}
	//Init response
	request.Response = &response
	// Handle the request: fill the response with info
	handleHTTPRequest(&request, server)

	//Write back on connection
	var writeBuffer bytes.Buffer
	request.Response.Write(&writeBuffer)
	con.Write(writeBuffer.Bytes())

	server.serverCondition.L.Lock()
	server.numConnections--
	if server.numConnections == MAX_CONNECTIONS-1 {
		server.serverCondition.Signal()
	}
	server.serverCondition.L.Unlock()
}

func getHTTPRequest(buffer []byte) http.Request {
	// Check if content exists
	reader := bytes.NewReader(buffer)
	bufReader := bufio.NewReader(reader)
	httpReq, err := http.ReadRequest(bufReader)

	if err != nil {
		log.Fatal(err)
		return http.Request{}
	}
	return *httpReq
}

func handleHTTPRequest(httpReq *http.Request, server *HTTPserver) {
	switch httpReq.Method {
	case "GET":
		makeGet(httpReq)
	case "POST":
		makePost(httpReq)
		server.tracer.Trace("File created: %s", httpReq.RequestURI)
	default:
		httpReq.Response.Status = "501 Not Implemented"
		httpReq.Response.StatusCode = 501
		server.tracer.Trace("Received non implemented request: %s", httpReq.Method)
	}
}

func makeGet(httpReq *http.Request) {
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
		log.Fatal(err)
		return
	}
	//fmt.Println("File", httpReq.Response.Header.Get("Content-Type"))
	// Try to open the file
	file, fileError := os.Open(workingDir + httpReq.RequestURI)

	if fileError != nil {
		httpReq.Response.Status = "404 Not Found"
		httpReq.Response.StatusCode = 404
		httpReq.Response.Header.Add("Content-Length", "0")
		return
	}

	fileInfo, infoError := file.Stat()

	if infoError != nil {
		log.Fatal(infoError)
	}
	httpReq.Response.Header.Add("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	httpReq.Response.Header.Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.Name()))
	httpReq.Response.Body = file
}

func makePost(httpReq *http.Request) {
	workingDir, werr := os.Getwd()
	if werr != nil {
		log.Fatal(werr)
		return
	}

	outFile, cerr := os.Create(workingDir + httpReq.RequestURI)
	if cerr != nil {
		log.Fatal(cerr)
	}
	defer outFile.Close()

	if _, cperr := io.Copy(outFile, httpReq.Body); cperr != nil {
		log.Fatal(cperr)
	}
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
			//fmt.Println("Woken up")
		}
		server.numConnections++
		if server.numConnections > MAX_CONNECTIONS {
			log.Fatal("Server capacity exceeded: EXPLODE")
		}

		server.tracer.Trace("Starting connectionHandler with %d concurrent connections\n", server.numConnections)

		server.serverCondition.L.Unlock()

		go connectionHandler(connection, &server)

		server.tracer.Trace("Total connections served: %d", server.numTotal)

	}

}
