# syntax=docker/dockerfile:1

FROM golang:1.23

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod ./
RUN go mod download


# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY . .

# Build
RUN go build -race ./main/mrcoordinator.go

# Run
CMD ["./mrcoordinator ./main/pg*txt"]