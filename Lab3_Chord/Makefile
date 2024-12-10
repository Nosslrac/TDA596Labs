

build: chordTypes.go chord.go rpc.go
	go build .

run: build
	./chord -a localhost -p 2134 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v -i 8f67a15066e9039bc6e6ace405ecff5c2680e5c5