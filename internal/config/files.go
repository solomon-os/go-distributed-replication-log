package config

import (
	"os"
	"path"
	"path/filepath"
)

var (
	CAFile               = certFile("ca.pem")
	ServerCertFile       = certFile("server.pem")
	ServerKeyFile        = certFile("server-key.pem")
	ClientCertFile       = certFile("client.pem")
	ClientKeyFile        = certFile("client-key.pem")
	RootClientCertFile   = certFile("root-client.pem")
	RootClientKeyFile    = certFile("root-client-key.pem")
	NobodyClientCertFile = certFile("nobody-client.pem")
	NobodyClientKeyFile  = certFile("nobody-client-key.pem")
	ACLModelFile         = aclFile("model.conf")
	ACLPolicyFile        = aclFile("policy.csv")
)

func certFile(filename string) string {
	if dir := os.Getenv("CERTS_DIR"); dir != "" {
		return filepath.Join(dir, filename)
	}

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return path.Join(dir, ".certs", filename)
}

func aclFile(filename string) string {
	if dir := os.Getenv("ACL_DIR"); dir != "" {
		return filepath.Join(dir, filename)
	}

	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path.Join(dir, ".", filename)
}
