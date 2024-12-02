# syntax=docker/dockerfile:1

FROM golang:1.23 AS builder

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download


# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY . .

# Build
RUN go build -race -buildmode=plugin ./mrapps/wc.go && \
	go build -race -buildmode=plugin ./mrapps/indexer.go  &&\
	go build -race -buildmode=plugin ./mrapps/mtiming.go &&\
	go build -race -buildmode=plugin ./mrapps/rtiming.go &&\
	go build -race -buildmode=plugin ./mrapps/jobcount.go &&\
	go build -race -buildmode=plugin ./mrapps/early_exit.go &&\
	go build -race -buildmode=plugin ./mrapps/crash.go &&\
	go build -race -buildmode=plugin ./mrapps/nocrash.go &&\
	go build -race ./main/mrworker.go


FROM alpine:latest
WORKDIR /root/
RUN apk add --no-cache libc6-compat ca-certificates tzdata
COPY --from=builder /app/mrworker /app/*.so ./

# Run
# ./mrworker ./mrapps/XXX.so <serverip>:<serverport> <workerport>
CMD ["./mrworker wc.so 127.0.0.1:1234 5555"]