# syntax=docker/dockerfile:1

FROM golang:1.23

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
	go build -race ./main/mrworker.go

# Run
# ./mrworker ./mrapps/XXX.so <serverip>:<serverport> <workerport>
CMD ["./mrworker wc.so 127.0.0.1:1234 5555"]