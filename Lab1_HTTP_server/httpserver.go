package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type HTTPserver struct{
	port string
	serverType string

}

type HTTPrequest struct{
	ReqType string // GET or POST
	File string    // File to GET or POST
	Proto string   // HTTP/1.0
	Content []byte // Only has information for POST requests
}

type HTTPresponse struct{
	Proto string   // HTTP/1.0
	status int		// Status code
	Response string // GET or POST
	ContentType string // Type
	ContentLen string // Length of content
	File string    // File to GET or POST
	Content []byte // Only has information for POST requests
}


func (response HTTPresponse) getByteData() []byte{
	answer := fmt.Sprintf("%s %d %s\r\n", response.Proto, response.status, response.Response)
	header := fmt.Sprintf("Content-Type: %s\r\nDate: %s", response.ContentType, time.Now().String())
	return []byte(answer + header)
}

func getArgs() HTTPserver {
	portvar := flag.String("port", "1234", "a port")
	flag.Parse()
	return HTTPserver{*portvar, "tcp"}
}


func connectionHandler(con net.Conn) {
	defer con.Close()
	readBuffer := make([]byte, 1024)
	n, err := con.Read(readBuffer)
	if err != nil {
		fmt.Println("Read failed", err)
		return
	}
	
	parsedReq := strings.Fields(string(readBuffer[:n]))
	response := getContent(parsedReq)

	con.Write(response.getByteData())
}




func getContent(request []string) HTTPresponse {
	// Check if content exists

	//Verify request
	if request[0] != "GET" {
		fmt.Println("No content to get")
		return HTTPresponse{"HTTP/1.1", 404, "NOT FOUND\r\n", 
		"", 
		"",
		"Host localhost", []byte{}}
	}

	contentType := "text/html; charset=UTF-8\r\n"
	contentLen := "0\r\n"

	resp := []byte{1, 2, 3, 4, 5, 6}
	return HTTPresponse{"HTTP/1.1", 200, "OK\r\n", 
	contentType, 
	contentLen,
	request[1],
	resp}
}


func main() {
	server := getArgs()
	fmt.Printf("Server started: listening on port: %s\n", server.port)
	
	listen, er := net.Listen(server.serverType, ":"+server.port)
	if er != nil {
		fmt.Println("Couldn't open port")
		os.Exit(-1)
	}
	defer listen.Close() //Close when l is handled
	
	//http.Serve(listen, nil)
	
	for {
		connection, err := listen.Accept()
		if err != nil {
			fmt.Println("Connection failed", err)
			continue
		}
		go connectionHandler(connection)
	}

	
}
