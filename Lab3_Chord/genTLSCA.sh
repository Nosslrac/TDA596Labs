mkdir -p cert
cd cert
rm -f *.pem

# 1. Generate CA's private key and self-signed certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes -keyout ca-key.pem -out ca-cert.pem -subj "/C=SE/ST=Vastra Gotaland/L=Gothenburg/O=TDA596Labs/OU=Lab3Chord/CN=Chord"

echo "CAs self-signed certificate generation complete"