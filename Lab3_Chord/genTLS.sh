
if [ ! -f "cert/ca-cert.pem" ]; then
    echo "CA cert not found: Run genTLSCA.sh first"
    exit 1 
fi

cd cert

# 2. Generate web server's private key and certificate signing request (CSR)
openssl req -newkey rsa:4096 -nodes -keyout server-key.pem -out server-req.pem -subj "/C=SE/ST=Vastra Gotaland/L=Gothenburg/O=TDA596Labs/OU=Lab3Chord/CN=Chord"

# 3. Use CA's private key to sign web server's CSR and get back the signed certificate
openssl x509 -req -in server-req.pem -days 60 -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem

echo "Server's signed certificate"

cat server-cert.pem ca-cert.pem > complete-cert.pem

cat server-cert.pem
# openssl x509 -in server-cert.pem -noout -text