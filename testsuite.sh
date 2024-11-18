#!/bin/bash
# Check that arguments are passed
if [[ -z "$1" || -z "$2" ]]
    then
    echo "Usage: ./testsuite.sh <serverip> <serverport> <proxyport>"
    echo "Proxy port may be omitted if the proxy is not being tested"
    exit 1
fi

serverip=$1
serverport=$2

proxyip="localhost"
proxyport=$3
awsserver="23.1.23.1"

out="TestsuiteResults"

rm -rf $out

printf "### Clean Testsuite files: start suite in 1 sec ###\n\n"
sleep 1
mkdir $out

printf "## Begin HTTP server test ## \n\n# Get file that doesn't exist #\n"
curl -i -X GET $serverip:$serverport/test.txt

printf "\n# Post the file that doesn't exist #\n"
curl -i -X POST $serverip:$serverport/test.txt -H "Content-Type: text/plain" --data-binary @input.txt

printf "\n# Get file again #\n"
curl -i -X GET $serverip:$serverport/test.txt

printf "\n# Post large file #\n"
curl -i -X POST $serverip:$serverport/largeFile.txt -H "Content-Type: text/plain" --data-binary @largeInput.txt

printf "\n# Large file #\n"
curl -s -D - -X GET $serverip:$serverport/largeFile.txt -o $out/fetchedLarge.txt

printf "\n# Non supported method #\n"
curl -i -X PUT $serverip:$serverport/test.txt

printf "\n# Get unsupported file #\n"
curl -i -X GET $serverip:$serverport/file.elf

printf "\n# Post image file #\n"
curl -i -X POST $serverip:$serverport/testImage.jpg -H "Content-Type: image/jpg" --data-binary @testImage.jpg

printf "\n# Get image file that was posted: output to fetchedImage.jpg #\n"
curl -s -D - -X GET $serverip:$serverport/testImage.jpg -o $out/fetchedImage.jpg

printf "\n## Bench mark server START ##\n"
ab -v 2 -n 1000 -c 100 $serverip:$serverport/test.txt > $out/serverBenchamark.txt
printf "\n## Server bench mark END: see $out/serverBenchamark.txt for results ##\n\n"

if [ -z "$3" ]
    then
    echo "Proxy not specified: Test suite finished"
    exit 0
fi

## Proxy testing
printf "## Begin PROXY test with same server ##\n\n"
printf "\n# Get file from that doesn't exist through proxy #\n"
curl -i -X GET $serverip:$serverport/notthere.txt -x $proxyip:$proxyport

printf "\n# Get file through proxy: added in previous test #\n"
curl -s -D - -o $out/proxyfetchedImage.jpg -X GET $serverip:$serverport/testImage.jpg -x $proxyip:$proxyport

printf "\n# Unsupported file formats will be sent to server which will respond accordingly #\n"
curl -i -X GET $serverip:$serverport/test.mp4 -x $proxyip:$proxyport

printf "\n# Post through proxy is not supported #\n"
curl -i -X POST $serverip:$serverport/test.txt -x $proxyip:$proxyport -H "Content-Type: text/plain" --data-binary @input.txt

printf "\n### Test suite finished: See terminal output and $out folder for results ###\n"
