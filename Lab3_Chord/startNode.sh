if [[ -z "$1" || -z "$2" ]]
    then
    echo "Usage: ./startNode.sh <serverip> <port>"
    exit 1
fi



docker run -p $2:$2 -v ./output:/app/:rw chord:latest ./chord -a $1 -p $2 --ts 2000 --tff 3000 --tcp 5000 -r 4 -v