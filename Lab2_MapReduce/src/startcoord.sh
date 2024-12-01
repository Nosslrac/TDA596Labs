#!/usr/bin/env bash

files=$(echo ./main/pg*txt)

if [ -z "$1" ]
    then
    docker run -v /repos/TDA596Labs/Lab2_MapReduce/src/output:/app/output:rw -p 1234:1234 coord:latest ./mrcoordinator $files
    #Verify that the output is correct
    sort output/mr-out* | grep . > mr-wc-all
    if cmp mr-wc-all mr-correct-wc.txt
    then
        echo '---' wc test: PASS
    else
        echo '---' wc output is not the same as mr-correct-wc.txt
        echo '---' wc test: FAIL
        exit 1
    fi
    exit 0
fi





