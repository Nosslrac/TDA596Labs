#!/bin/bash
# Check that arguments are passed

blue=$(tput setaf 3)
red=$(tput setaf 1)
normal=$(tput sgr0)

if [[ -z "$1" || -z "$2" ]]
    then
    echo "${red}Usage: ./testsuite.sh <serverip> <serverport> <proxyport>"
    echo "Proxy port may be omitted if the proxy is not being tested"
    exit 1
fi

serverip=$1
serverport=$2

proxyip=$3
proxyport=$4
awsserver="23.1.23.1"

out="TestsuiteResults"

rm -rf $out

printf "${blue}### Clean Testsuite files: start suite in 1 sec ###\n\n${normal}"
sleep 1
mkdir $out

printf "${blue}## Begin HTTP server test ## \n\n# Get file that doesn't exist #\nRequest line: GET $serverip:$serverport/test.txt\n\n${normal}"
curl -i -X GET $serverip:$serverport/test.txt

printf "${blue}\n# Post the file that doesn't exist #\nRequest line: POST $serverip:$serverport/test.txt\n\n${normal}"
curl -i -X POST $serverip:$serverport/test.txt -H "Content-Type: text/plain" --data-binary @input.txt

printf "${blue}\n# Get file again #\nRequest line: GET $serverip:$serverport/test.txt\n\n${normal}"
curl -i -X GET $serverip:$serverport/test.txt

printf "${blue}\n# Post large file #\nRequest line: POST $serverip:$serverport/largeFile.txt\n\n${normal}"
curl -i -X POST $serverip:$serverport/largeFile.txt -H "Content-Type: text/plain" --data-binary @largeInput.txt

printf "${blue}\n# Large file #\nRequest line: POST $serverip:$serverport/largeFile.txt\n\n${normal}"
curl -s -D - -X GET $serverip:$serverport/largeFile.txt -o $out/fetchedLarge.txt

printf "${blue}\n# Non supported method #\nRequest line: PUT $serverip:$serverport/test.txt\n${normal}"
curl -i -X PUT $serverip:$serverport/test.txt -H "Content-Type: text/plain" --data-binary @input.txt

printf "${blue}\n# Get unsupported file #\nRequest line: GET $serverip:$serverport/file.elf\n\n${normal}"
curl -i -X GET $serverip:$serverport/file.elf

printf "${blue}\n# Post image file #\nRequest line: POST $serverip:$serverport/testImage.jpg\n\n${normal}"
curl -i -X POST $serverip:$serverport/testImage.jpg -H "Content-Type: image/jpg" --data-binary @testImage.jpg

printf "${blue}\n# Get image file that was posted: output to fetchedImage.jpg #\nRequest line: GET $serverip:$serverport/testImage.jpg\n\n${normal}"
curl -s -D - -X GET $serverip:$serverport/testImage.jpg -o $out/fetchedImage.jpg

printf "${blue}\n## Bench mark server START ##\n\n${normal}"
ab -v 2 -n 200 -c 20 $serverip:$serverport/test.txt > $out/serverBenchamark.txt
printf "${blue}\n## Server bench mark END: see $out/serverBenchamark.txt for results ##\n\n\n${normal}"

if [ -z "$3" ]
    then
    echo "${red}Proxy not specified: Test suite finished"
    exit 0
fi

## Proxy testing
printf "${blue}## Begin PROXY test with same server ##\n\n\n${normal}"
printf "${blue}\n# Get file from that doesn't exist through proxy #\nRequest line: GET $serverip:$serverport/notthere.txt\n\n${normal}"
curl -i -X GET $serverip:$serverport/notthere.txt -x $proxyip:$proxyport

printf "${blue}\n# Get file through proxy: added in previous test #\nRequest line: GET $serverip:$serverport/testImage.jpg\n\n${normal}"
curl -s -D - -o $out/proxyfetchedImage.jpg -X GET $serverip:$serverport/testImage.jpg -x $proxyip:$proxyport

printf "${blue}\n# Unsupported file formats will be sent to server which will respond accordingly #\nRequest line: GET $serverip:$serverport/test.mp4\n\n${normal}"
curl -i -X GET $serverip:$serverport/test.mp4 -x $proxyip:$proxyport

printf "${blue}\n# Post through proxy is not supported #\nRequest line: POST $serverip:$serverport/test.txt\n\n${normal}"
curl -i -X POST $serverip:$serverport/test.txt -x $proxyip:$proxyport -H "Content-Type: text/plain" --data-binary @input.txt

printf "${blue}\n### Test suite finished: See terminal output and $out folder for results ###\n\n${normal}"
