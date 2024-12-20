
if [ -z "$1" ]
    then
    echo "Usage: ./startchord.sh <port>"
    exit 1
fi

workerip=$(curl ifconfig.me)

docker run -it -p $1:$1 -v ./output:/app/output:rw nosslrac/chord ./chord -a $workerip -p $1 --ts 300 --tff 500 --tcp 1000 -r 4 -v

exit 0

if [ -z "$1" ]
    then
    echo "Usage: ./startchord.sh <port>"
    exit 1
fi

workerip=$(curl ifconfig.me)

docker run -it -p $1:$1 -v ./output:/app/output:rw nosslrac/chord ./chord -a $workerip -p $1 --ts 300 --tff 500 --tcp 1000  -r 4 -v -ja 3.89.91.14 -jp 1111