package server

import (
	"context"
	"errors"
	"strings"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	// grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	api "github.com/solomon-os/go-distributed-replication-log/log/api/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Config struct {
	CommitLog  CommitLog
	Authorizer Authorizer
}

const (
	objectWildcard = "*"
	produceAction  = "produce"
	consumeAction  = "consume"
)

var _ api.LogServiceServer = (*grpcServer)(nil)

type grpcServer struct {
	api.UnimplementedLogServiceServer
	*Config
}

func NewGRPCServer(config *Config, opts ...grpc.ServerOption) (*grpc.Server, error) {
	logger := zap.L().Named("server")
	zapOtps := []grpc_zap.Option{
		grpc_zap.WithDurationField(
			func(duration time.Duration) zapcore.Field {
				return zap.Int64("grpc.time_ns", duration.Nanoseconds())
			},
		),
	}

	// trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	halfSampler := trace.ProbabilitySampler(0.5)
	trace.ApplyConfig(trace.Config{
		DefaultSampler: func(sp trace.SamplingParameters) trace.SamplingDecision {
			if strings.Contains(sp.Name, "Produce") {
				return trace.SamplingDecision{Sample: true}
			}

			return halfSampler(sp)
		},
	})

	err := view.Register(ocgrpc.DefaultServerViews...)
	if err != nil {
		return nil, err
	}

	opts = append(opts, grpc.StreamInterceptor(
		grpc_middleware.ChainStreamServer(
			grpc_ctxtags.StreamServerInterceptor(),
			grpc_zap.StreamServerInterceptor(logger, zapOtps...),
			grpc_auth.StreamServerInterceptor(authenticate),
		)),

		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_ctxtags.UnaryServerInterceptor(),
			grpc_zap.UnaryServerInterceptor(logger, zapOtps...),
			grpc_auth.UnaryServerInterceptor(authenticate),
		)),

		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
	)

	gsrv := grpc.NewServer(opts...)
	srv, err := newGrpcServer(config)
	if err != nil {
		return nil, err
	}
	api.RegisterLogServiceServer(gsrv, srv)
	return gsrv, nil
}

func newGrpcServer(config *Config) (srv *grpcServer, err error) {
	srv = &grpcServer{
		Config: config,
	}
	return srv, nil
}

func (s *grpcServer) Produce(
	ctx context.Context,
	req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
	if err := s.Authorizer.Authorize(subject(ctx), objectWildcard, produceAction); err != nil {
		return nil, err
	}

	off, err := s.CommitLog.Append(req.Record)
	if err != nil {
		return nil, err
	}

	return &api.ProduceResponse{Offset: off}, nil
}

func (s *grpcServer) Consume(
	ctx context.Context,
	req *api.ConsumeRequest,
) (*api.ConsumeResponse, error) {
	if err := s.Authorizer.Authorize(subject(ctx), objectWildcard, consumeAction); err != nil {
		return nil, err
	}

	record, err := s.CommitLog.Read(req.Offset)
	if err != nil {
		return nil, err
	}
	return &api.ConsumeResponse{Record: record}, nil
}

func (s *grpcServer) ProduceStream(stream api.LogService_ProduceStreamServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		res, err := s.Produce(stream.Context(), req)
		if err != nil {
			return err
		}
		if err = stream.Send(res); err != nil {
			return err
		}
	}
}

func (s *grpcServer) ConsumeStream(
	req *api.ConsumeRequest,
	stream api.LogService_ConsumeStreamServer,
) error {
	for {
		select {
		case <-stream.Context().Done():
			return nil
		default:
			res, err := s.Consume(stream.Context(), req)
			if err != nil {
				if _, ok := errors.AsType[api.ErrOffsetOutOfRange](err); ok {
					continue
				}
				return err
			}

			if err = stream.Send(res); err != nil {
				return err
			}
			req.Offset++
		}
	}
}

type CommitLog interface {
	Append(*api.Record) (uint64, error)
	Read(uint64) (*api.Record, error)
}

type Authorizer interface {
	Authorize(subject, object, action string) error
}

func authenticate(ctx context.Context) (context.Context, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return ctx, status.New(codes.Unknown, "couldn't find peer info").Err()
	}
	if peer.AuthInfo == nil {
		return context.WithValue(ctx, subjectContextKey{}, ""), nil
	}
	tlsInfo := peer.AuthInfo.(credentials.TLSInfo)
	subject := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName
	ctx = context.WithValue(ctx, subjectContextKey{}, subject)

	return ctx, nil
}

func subject(ctx context.Context) string {
	return ctx.Value(subjectContextKey{}).(string)
}

type subjectContextKey struct{}
