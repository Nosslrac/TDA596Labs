RACE=-race

runc: build
	./mrcoordinator ./pg*txt

runw1:
	./mrworker ../mrapps/wc.so 18.208.146.146:1234 5555
	
runw2:
	./mrworker ../mrapps/wc.so 127.0.0.1:1234 182.23.3.192:2223

build: clean
	cd ../mrapps && go build $(RACE) -buildmode=plugin wc.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin indexer.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin jobcount.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin mtiming.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin nocrash.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin rtiming.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin early_exit.go
	cd ../mrapps && go build $(RACE) -buildmode=plugin crash.go
	go build $(RACE) mrcoordinator.go
	go build $(RACE) mrworker.go
	go build $(RACE) mrsequential.go
	

clean:
	rm -f mr-*-*
	rm -f mr-out*
	cd ../mrapps && go clean
	cd .. && go clean

dockerc:
	docker run -v ./output:/app/ -p 1234:1234 coord:latest ./mrcoordinator 

dockerit:
	docker run -it -p 1234:1234 coord:latest sh