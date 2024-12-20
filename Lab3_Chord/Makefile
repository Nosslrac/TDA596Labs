
files=$(echo *.go)

build: chordTypes.go chord.go rpc.go
	go build -race -o chord

run: build cert
	./chord -a localhost -p 1111 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v

runa: build cert
	./chord -a localhost -p 2222 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

runb: build cert
	./chord -a localhost -p 3333 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

runc: build cert
	./chord -a localhost -p 4444 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

rund: build cert
	./chord -a localhost -p 5555 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

rune: build cert
	./chord -a localhost -p 6666 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

runf: build cert
	./chord -a localhost -p 7777 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

rung: build cert
	./chord -a localhost -p 8888 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -jp 1111 -ja localhost

cert:
	chmod +x genTLS.sh; ./genTLS.sh

.PHONY: cert