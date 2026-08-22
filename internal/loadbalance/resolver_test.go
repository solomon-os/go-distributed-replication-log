package loadbalance_test

import (
	"net"
	"net/url"
	"testing"

	"github.com/solomon-os/go-distributed-replication-log/log/internal/config"
	"github.com/solomon-os/go-distributed-replication-log/log/internal/loadbalance"
	"github.com/solomon-os/go-distributed-replication-log/log/internal/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/resolver"
)

func TestResolver(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	tlsConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerCertFile,
		CAFile:        config.CAFile,
		Server:        true,
		ServerAddress: l.Addr().String(),
	})
	require.NoError(t, err)
	serverCreds := credentials.NewTLS(tlsConfig)
	srv, err := server.NewGRPCServer(
		&server.Config{GetServerer: &getServers{}},
		grpc.Creds(serverCreds),
	)
	require.NoError(t, err)

	go srv.Serve(l)

	conn := &clientConn{}
	tlsConfig, err = config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.RootClientCertFile,
		KeyFile:       config.RootClientKeyFile,
		CAFile:        config.CAFile,
		Server:        false,
		ServerAddress: l.Addr().String(),
	})
	require.NoError(t, err)

	clientCreds := credentials.NewTLS(tlsConfig)
	opts := resolver.BuildOptions{
		DialCreds: clientCreds,
	}
	r := &loadbalance.Resolver{}

	_, err = r.Build(
		resolver.Target{
			URL: url.URL{Host: l.Addr().String()},
		},
		conn,
		opts,
	)
}
