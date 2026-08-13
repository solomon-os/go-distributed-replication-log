CONFIG_PATH ?=${PWD}/.certs

init: 
	mkdir -p ${CONFIG_PATH}

gencert: init
	cfssl gencert \
		-initca cert-config/ca-csr.json | cfssljson -bare ca;
	cfssl gencert \
		-ca=./ca.pem \
		-ca-key=./ca-key.pem \
		-config=cert-config/ca-config.json \
		-profile=server \
		cert-config/server-csr.json | cfssljson -bare server
	cfssl gencert \
		-ca=./ca.pem \
		-ca-key=./ca-key.pem \
		-config=cert-config/ca-config.json \
		-profile=client \
		cert-config/client-csr.json | cfssljson -bare client
	mv *.pem *.csr ${CONFIG_PATH} 

test: 
	CONFIG_DIR=${CONFIG_PATH} go test -v -race ./...
