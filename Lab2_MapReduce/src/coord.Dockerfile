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
RUN go build -race ./main/mrcoordinator.go

FROM alpine:latest
WORKDIR /app/
RUN apk add --no-cache libc6-compat ca-certificates tzdata

#Add the necessary files to run the coordinator
COPY --from=builder /app/mrcoordinator /app/main/pg*txt ./

# Run
CMD ["./mrcoordinator ./pg*txt"]