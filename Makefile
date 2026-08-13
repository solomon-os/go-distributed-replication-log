CONFIG_PATH=./.certs

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
	mv *.pem *.csr ${CONFIG_PATH} 

test test: go test -race ./...
