RACE=-race

runc: build
	./mrcoordinator ./pg*txt

runw1:
	./mrworker ../mrapps/wc.so 127.0.0.1:1234 5555
	
runw2:
	./mrworker ../mrapps/wc.so 127.0.0.1:1234 4444

build: clean
	cd ../mrapps && go build $(RACE) -buildmode=plugin wc.go
	go build $(RACE) mrcoordinator.go
	go build $(RACE) mrworker.go
	go build $(RACE) mrsequential.go
	

clean:
	rm -f mr-*-*
	rm -f mr-out*
	cd ../mrapps && go clean
	cd .. && go clean