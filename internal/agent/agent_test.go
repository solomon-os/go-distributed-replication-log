package agent_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"testing"
	"time"

	api "github.com/solomon-os/go-distributed-replication-log/log/api/v1"
	"github.com/solomon-os/go-distributed-replication-log/log/internal/agent"
	"github.com/solomon-os/go-distributed-replication-log/log/internal/config"
	"github.com/solomon-os/go-distributed-replication-log/log/internal/loadbalance"
	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func TestAgent(t *testing.T) {
	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerKeyFile,
		CAFile:        config.CAFile,
		Server:        true,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	peerTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      config.RootClientCertFile,
		KeyFile:       config.RootClientKeyFile,
		CAFile:        config.CAFile,
		Server:        false,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	var agents []*agent.Agent
	for i := range 3 {
		ports := dynaport.Get(2)
		bindAddr := fmt.Sprintf("%s:%d", "127.0.0.1", ports[0])
		rpcPort := ports[1]

		dataDir, err := os.MkdirTemp("", "agent-test-log")
		require.NoError(t, err)

		var startJoinAddrs []string
		if i != 0 {
			startJoinAddrs = append(startJoinAddrs, agents[0].Config.BindAddr)
		}

		agent, err := agent.New(
			agent.Config{
				Bootstrap:       i == 0,
				NodeName:        fmt.Sprintf("%d", i),
				StartJoinAddrs:  startJoinAddrs,
				BindAddr:        bindAddr,
				RPCPort:         rpcPort,
				DataDir:         dataDir,
				ACLModelFile:    config.ACLModelFile,
				ACLPolicyFile:   config.ACLPolicyFile,
				ServerTLSConfig: serverTLSConfig,
				PeerTLSConfig:   peerTLSConfig,
			},
		)

		require.NoError(t, err)
		agents = append(agents, agent)
	}

	defer func() {
		for _, agent := range agents {
			err := agent.Shutdown()
			require.NoError(t, err)
			require.NoError(t, os.RemoveAll(agent.Config.DataDir))
		}
	}()
	time.Sleep(3 * time.Second)

	leaderClient := client(t, agents[0], peerTLSConfig)

	produceResponse, err := leaderClient.Produce(
		context.Background(),
		&api.ProduceRequest{Record: &api.Record{Value: []byte("foo")}},
	)
	require.NoError(t, err)

	// wait until replication has finished
	time.Sleep(3 * time.Second)

	consumeReponse, err := leaderClient.Consume(
		context.Background(),
		&api.ConsumeRequest{Offset: produceResponse.Offset},
	)
	require.NoError(t, err)
	require.Equal(t, consumeReponse.Record.Value, []byte("foo"))

	followerClient := client(t, agents[1], peerTLSConfig)
	consumeReponse, err = followerClient.Consume(
		context.Background(),
		&api.ConsumeRequest{Offset: produceResponse.Offset},
	)

	require.NoError(t, err)
	require.Equal(t, consumeReponse.Record.Value, []byte("foo"))

	consumeReponse, err = leaderClient.Consume(context.Background(), &api.ConsumeRequest{
		Offset: produceResponse.Offset + 1,
	})
	require.Nil(t, consumeReponse)
	require.Error(t, err)
	got := status.Code(err)
	want := status.Code(api.ErrOffsetOutOfRange{}.GRPCStatus().Err())
	require.Equal(t, got, want)
}

func client(t *testing.T, agent *agent.Agent, tlsConfig *tls.Config) api.LogServiceClient {
	tlsCreds := credentials.NewTLS(tlsConfig)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}
	rpcAddr, err := agent.Config.RPCAddr()
	require.NoError(t, err)

	conn, err := grpc.NewClient(fmt.Sprintf("%s://%s", loadbalance.Name, rpcAddr), opts...)
	require.NoError(t, err)

	return api.NewLogServiceClient(conn)
}
