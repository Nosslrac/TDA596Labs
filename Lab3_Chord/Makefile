

build: chordTypes.go chord.go rpc.go
	go build .

run: build
	./chord -a localhost -p 1234 --ts 2000 --tff 3000 --tcp 5000 -r 1 -v

runs: build
	./chord -a localhost -p 2341 --ts 2000 --tff 3000 --tcp 5000 -r 1 -v -jp 1234 -ja localhost

runt: build
	./chord -a localhost -p 3452 --ts 2000 --tff 3000 --tcp 5000 -r 1 -v -jp 1234 -ja localhost