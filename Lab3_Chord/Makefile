
files=$(echo *.go)

build: chordTypes.go chord.go rpc.go
	go build -race -o chord

run: build cert
	./chord -a localhost -p 1111 --ts 2000 --tff 5000 --tcp 5000 -r 4 -v

runs: build cert
	./chord -a localhost -p 3333 --ts 2000 --tff 5000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

runt: build cert
	./chord -a localhost -p 5555 --ts 2000 --tff 5000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

runb: build cert
	./chord -a localhost -p 4444 --ts 2000 --tff 5000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

cert:
	chmod +x genTLS.sh; ./genTLS.sh

.PHONY: cert