# syntax=docker/dockerfile:1

FROM golang:1.23 AS builder

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod ./
RUN go mod download


# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY . .

# Build
RUN go build -race -o chord

FROM alpine:latest
WORKDIR /app/
# RUN apk add --no-cache libc6-compat ca-certificates tzdata

#Add the necessary files to run the coordinator
COPY --from=builder /app/chord /app/*.txt /app/cert/ca-cert.pem /app/cert/ca-key.pem /app/genTLS.sh ./

# Run
CMD ["./chord -a localhost -p 1111 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v"]