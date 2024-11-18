# TDA596Labs
The labs of the chalmers course TDA596

## How to build

To build the HTTP server simpliy navigate to ```Lab1_HTTP_server``` folder, standing in the repository root:

```sh
cd Lab1_HTTP_server
go build .
```
This will generate a binary called ```httpserver```, which could be run like below:
```sh
./httpserver -port=2222 -v
```
**Note:** the flags are optional. ```-v``` runs the server in verbose mode, providing helpful prints for debugging. ```-port=<portnumber>``` specifies which port the server is run on, default is 1234 if this flag is omitted.

To build the proxy simpliy navigate to ```Proxy``` folder, standing in the repository root:
```sh
cd Lab1_HTTP_server/Proxy
go build .
```

The proxy is run is run in a very similar fashion as the HTTP server and the same flags apply.

## Using Docker
You need to have docker installed. To build the HTTP server navigate to ```Lab1_HTTP_server``` folder and run the following command:
```sh
docker build -t httpserver:latest .
docker run -p <externalport>:<internalport> httpserver:latest ./httpserver -port=<internalport>
e.g.
docker run -p 5555:1010 httpserver:latest ./httpserver -port=1010
```
This will start the server on internal port 1010 which maps to the external port 5555. Starting the proxy can be done similarily:
```sh
cd $(REPOROOT)/Lab1_HTTP_server/Proxy
docker build -t proxy:latest .
docker run -p <externalport>:<internalport> proxy:latest ./proxy -port=<internalport>
e.g.
docker run -p 3333:1222 proxy:latest ./proxy -port=1222
```
When dockerizing the programs, the intended use is that the proxy and server are run on different machines.

**IMPORTANT:** If the containers are running on the same system you can use the docker compose file:
```sh
docker compose up
```
Since they are running on the same system, you will have to use internal docker service names for inter-container communication. 
See in the Testsuite section how ```testsuite.sh``` can be run to account for this.


## Working requests
The server only implements GET and POST all other requests will illicit a ```510 Not Implemented```.

## Testing the server
```curl``` and ```ab``` (from apache2-utils ```sudo apt install apache2-utils```) can be used to test the server and proxy. ```ab``` good for benchmarking and to make sure that your server doesn't exceed capacity when receiving multiple requests concurrently.

### Using curl
```sh
curl -v -X GET <serverip>:<port>/<file>
```

Testing GET:
```sh
curl -v -X GET localhost:1234/test.txt
```

Testing POST (-v for verbosity, -H for headers, use --help for info about curl):
```sh
curl -v -X POST localhost:1234/this.jpg -H "Content-Type: image/jpg" --data-binary @tmp.jpg
```

Testing the proxy aswell:
```sh
curl -v -X GET localhost:1234/test.txt -x localhost:5555
```

### Using ab
Structure of the command (use ab -h to get info about the command):
```sh
ab [options] [http[s]://]hostname[:port]/path
```

Testing the server by POSTing test.jpg 20 times, 10 at the time:
```sh
ab -v 2 -n 20 -c 10 -p tmp.jpg -T image/jpg localhost:1234/test.jpg
```
Benchmarking the server by GETing test.txt 100 times 20 requests at the time
```sh
ab -v 2 -n 100 -c 20 localhost:1234/test.txt
```

## Testsuite usage
The server also comes with a testsuite which is run by specifying server ip and port aswell as proxy ip and port. Proxy information can be omitted if only the server is to be tested.
```sh
./testsuite.sh <serverip> <serverport> <proxyip> <proxyport>
```
For example
```sh
./testsuite.sh localhost 5555 localhost 2222
```


**IMPORTANT:** If the proxy and HTTP server run as docker containers on the same machine then you will have to use the container names for inter-container communication.
With the current ```docker-compose.yml``` the test can be run as follows:
```sh
./testsuite.sh localhost 3333 localhost 5555 same
```
where same specifies that the proxy should use internal container name to communicate with the server.


### Dependencies
- curl
- ab

## AWS instructions

- Download labsuser.pem to .ssh/ and chmod 400
```sh
chmod ~/.ssh/labsuser.pem 400
ssh -i ~/.ssh/labsuser.pem ec2-user@<public-ip>
```
- Set aws cli on both local machine and EC2 instance ctrl+c on AWS details:
```sh
vim ~/.aws/credentials
esc+dd dG
ctrl+v
```
- On EC2 instance: fetch newest docker image from ECR
- Login with docker on EC2 instance
```sh
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin <user-id>.dkr.ecr.us-east-1.amazonaws.com
```
then do pull the latest image from ECR.
- Pull docker image:
```sh
docker pull <URI>
```
- Run with e.g.:
```sh
docker run -p 5555:1010 <dockerimg> ./httpserver -port=1010
```
