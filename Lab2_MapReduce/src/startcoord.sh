#!/usr/bin/env bash


docker run -v ./output:/app/output:rw -p 1234:1234 coord:latest ./mrcoordinator ./pg-being_ernest.txt ./pg-dorian_gray.txt ./pg-frankenstein.txt ./pg-grimm.txt ./pg-huckleberry_finn.txt ./pg-metamorphosis.txt ./pg-sherlock_holmes.txt ./pg-tom_sawyer.txt
#Verify that the output is correct

sort output/mr-out* | grep . > mr-wc-all

if [ -z "$1" ]
then
        echo "Usage: ./startcoord.sh <verifyId>"
        echo "verfyId=1 -> wc.so"
        echo "verfyId=2 -> indexer.so"
        echo "verfyId=3 -> crash.so"
        echo "verfyId=4 -> jobcount.so"
        echo "verfyId=4 -> mtiming.so"
fi

if cmp mr-wc-all mr-correct-$1.txt
then
    echo '---' wc test: PASS
else
    echo '---' wc output is not the same as mr-correct-wc.txt
    echo '---' wc test: FAIL
    exit 1
fi