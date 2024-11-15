INTERNALPORT=1010
EXTERNALPORT=5555

logindocker:
	aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 654654237508.dkr.ecr.us-east-1.amazonaws.com

#aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin <user-id>.dkr.ecr.us-east-1.amazonaws.com

# See ecr repo for push commands ( will get login)
pushcommands:

#docker pull <URI>

#With path to pem file
dossh:
	ssh -i <path/to/filename>.pem ec2-user@<public-ip>

run:
	docker run -d -p 5555:1010 httpserver:latest ./httpserver -port=1010
#                external:internal    							    internal



## When port is blocked
#docker kill <container>