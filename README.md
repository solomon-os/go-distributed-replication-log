# Distributed Log Service

This project is a distributed log service written in Go. It shows how systems such as Kafka store messages, copy data across multiple servers, and continue working as a cluster.

A client can add a record to the log, read a record using its offset, stream records, and ask for the list of servers in the cluster.

## What I implemented

- A persistent commit log that stores records on disk.
- Log segments and a memory-mapped index for finding records quickly.
- Automatic creation of a new segment when the current one becomes full.
- A gRPC API for producing and consuming records.
- Streaming RPCs for sending or reading several records.
- Raft consensus for choosing a leader and replicating data between nodes.
- Automatic node discovery and membership management with Serf.
- A custom gRPC load balancer. Write requests go to the leader, while read requests are shared between followers.
- TLS encryption and client certificate authentication.
- Role-based authorization with Casbin. The included policy allows the `root` client to produce and consume records.
- Logging, metrics, tracing, and a gRPC health check.
- Unit and integration tests for storage, the API, discovery, replication, agents, and load balancing.
- A multi-stage Docker image.
- Helm charts for running a three-node cluster in Kubernetes with persistent storage and health probes.

## How it works

Each record is saved with an offset. The offset is its position in the log and can be used to read the record later.

### Memory mapping

I used memory mapping for each segment's index file. Memory mapping lets the program work with a file as if it were part of memory, while the operating system handles moving the data between memory and disk.

The index stores each record's relative offset and its exact position in the data file. This means the service can find a record directly instead of reading through the whole log. Index changes are synced to disk when the index closes, and the program can recover the valid index entries after an unclean shutdown.

When a client writes a record, the request is sent to the Raft leader. Raft copies the change to the other nodes before it is committed. Serf lets nodes discover when another node joins or leaves, and the cluster updates its Raft membership.

The gRPC server and Raft traffic share one TCP port. The service separates the traffic internally and can protect both client and node connections with TLS.

## Main API operations

- `Produce` adds one record and returns its offset.
- `Consume` reads one record from an offset.
- `ProduceStream` adds records through a two-way stream.
- `ConsumeStream` continuously reads records starting at an offset.
- `GetServers` returns the cluster nodes and shows which one is the leader.

The API definition is in `api/v1/log.proto`.

## Project structure

```text
api/v1/              Protocol Buffer and generated gRPC code
cmd/proglog/         Main server command
cmd/getservers/      Small client that lists cluster servers
internal/log/        Storage, segments, indexes, Raft, and replication
internal/server/     gRPC server and middleware
internal/agent/      Connects the server, log, Raft, and discovery parts
internal/discovery/  Serf cluster membership
internal/auth/       Casbin authorization
internal/loadbalance/ Custom gRPC resolver and request picker
internal/config/     TLS and file configuration
deploy/              Kubernetes Helm charts and hooks
cert-config/         Certificate generation settings
```

## Running the tests

The project uses Go 1.26.1. The security tests also need test certificates.

Install `cfssl` and `cfssljson`, then run:

```bash
make gencert
make test-all
```

To run one test by name:

```bash
make test RUN=TestAgent
```

Set `DEBUG=true` if you want extra logs while testing:

```bash
make test-all DEBUG=true
```

## Running one node locally

First generate the certificates with `make gencert`. Then start a single-node cluster:

```bash
go run ./cmd/proglog \
  --data-dir=/tmp/proglog-node-1 \
  --node-name=node-1 \
  --bind-addr=127.0.0.1:8401 \
  --rpc-port=8400 \
  --bootstrap \
  --acl-model-file=./model.conf \
  --acl-policy-file=./policy.csv \
  --server-tls-cert-file=./.certs/server.pem \
  --server-tls-key-file=./.certs/server-key.pem \
  --server-tls-ca-file=./.certs/ca.pem
```

The server keeps running until it receives `Ctrl+C`. A real client must use a trusted client certificate because produce and consume requests are protected by mutual TLS and the ACL policy.

## Docker

Build the server image with:

```bash
make build-docker TAG=0.0.1
```

The final image contains only the server binary, the gRPC health probe, and the ACL files.

## Kubernetes

The Helm chart in `deploy/proglog` creates a StatefulSet with three nodes by default. Each node has its own persistent volume. The first node starts the cluster, and the other nodes join it automatically.

Before installing the chart, update the image repository and tag in `deploy/proglog/values.yaml` so Kubernetes can pull your image.

```bash
helm install proglog ./deploy/proglog
```

The deployment also includes readiness and liveness checks. The optional MetaController files create a separate service for each pod when per-pod load balancing is enabled.

## What I learned

This project helped me understand how a distributed service handles storage, leader election, replication, service discovery, secure communication, access control, load balancing, observability, testing, and Kubernetes deployment.
