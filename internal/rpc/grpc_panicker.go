package rpc

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/DIMO-Network/meta-transaction-processor/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCMetricsAndLogMiddleware tracks error and success prom metrics for grpc, and also logs if there is an error
func GRPCMetricsAndLogMiddleware(logger *zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		resp, err := handler(ctx, req)

		if err != nil {
			if s, ok := status.FromError(err); ok {
				metrics.GRPCResponseTime.With(prometheus.Labels{"method": info.FullMethod, "status": s.Code().String()}).Observe(time.Since(startTime).Seconds())
				metrics.GRPCRequestCount.With(prometheus.Labels{"method": info.FullMethod, "status": s.Code().String()}).Inc()

				logger.Err(err).Str("grpc_status_code", s.Code().String()).Str("grpc_method", info.FullMethod).Msg("grpc request error")
			} else {
				metrics.GRPCResponseTime.With(prometheus.Labels{"method": info.FullMethod, "status": "unknown"}).Observe(time.Since(startTime).Seconds())
				metrics.GRPCRequestCount.With(prometheus.Labels{"method": info.FullMethod, "status": "unknown"}).Inc()

				logger.Err(err).Str("grpc_status_code", "unknown").Str("grpc_method", info.FullMethod).Msg("grpc request error")
			}
		} else {
			metrics.GRPCResponseTime.With(prometheus.Labels{"method": info.FullMethod, "status": "OK"}).Observe(time.Since(startTime).Seconds())
			metrics.GRPCRequestCount.With(prometheus.Labels{"method": info.FullMethod, "status": "OK"}).Inc()
		}

		return resp, err
	}
}

type GRPCPanicker struct {
	Logger *zerolog.Logger
}

func (pr *GRPCPanicker) GRPCPanicRecoveryHandler(p any) (err error) {
	metrics.GRPCPanicsCount.Inc()

	pr.Logger.Err(fmt.Errorf("%s", p)).Str("stack", string(debug.Stack())).Msg("grpc recovered from panic")
	return status.Errorf(codes.Internal, "%s", p)
}
