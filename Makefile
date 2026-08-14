CERTS_DIR ?=${PWD}/.certs

init: 
	mkdir -p ${CERTS_DIR}

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
		-cn="root" \
		cert-config/client-csr.json | cfssljson -bare root-client
	cfssl gencert \
		-ca=./ca.pem \
		-ca-key=./ca-key.pem \
		-config=cert-config/ca-config.json \
		-profile=client \
		-cn="nobody" \
		cert-config/client-csr.json | cfssljson -bare nobody-client
	mv *.pem *.csr ${CERTS_DIR} 

test: 
	CERTS_DIR=${CERTS_DIR} ACL_DIR=${PWD}/ go test -v -race ./...
