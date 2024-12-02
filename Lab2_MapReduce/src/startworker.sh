#!/usr/bin/env bash

if [[ -z "$1" || -z "$2" ]]
    then
    echo "Usage: startworker.sh xxx.so <serverip:port>"
    exit 1
fi

port=2222
workerip=$(curl ifconfig.me):$port

#echo docker run -p $port:$port worker:latest ./mrworker $1 $2 $workerip
docker run -p $port:$port worker:latest ./mrworker $1 $2 $workerip