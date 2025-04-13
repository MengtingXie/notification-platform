package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Builder struct {
	// apiDurationHistogram tracks API response times
	apiDurationHistogram *prometheus.HistogramVec
}

// New creates a new Builder with initialized metrics
func New() *Builder {
	return &Builder{
		apiDurationHistogram: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "grpc_server_handling_seconds",
				Help:    "Histogram of response latency (seconds) of gRPC requests.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "status"},
		),
	}
}

func (b *Builder) Build() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Record start time
		startTime := time.Now()

		// Process the request
		resp, err := handler(ctx, req)

		// Calculate duration
		duration := time.Since(startTime).Seconds()

		// Get status code
		st, _ := status.FromError(err)
		statusCode := st.Code().String()

		// Report to Prometheus
		b.apiDurationHistogram.WithLabelValues(
			info.FullMethod,
			statusCode,
		).Observe(duration)

		return resp, err
	}
}
