
runc: build
	./mrcoordinator ./pg*txt

runw:
	./mrworker wc.so

build: clean
	go build -race -buildmode=plugin ../mrapps/wc.go
	cp ../mrapps/wc.so ./
	go build -race mrcoordinator.go
	go build -race mrworker.go
	
clean:
	go clean
	rm -f mr-*-*
	rm -f mr-out*