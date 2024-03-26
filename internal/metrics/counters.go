package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TicksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "meta_transaction_processor",
		Name:      "ticks_total",
	})

	TickErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "meta_transaction_processor",
		Name:      "tick_errors_total",
	})

	GRPCPanicsCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "meta_transaction_processor_panics_total",
		Help: "Total Panics recovered",
	})

	GRPCRequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meta_transaction_processor_grpc_request_count",
			Help: "The total number of requests served by the GRPC Server",
		},
		[]string{"method", "status"},
	)

	GRPCResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "meta_transaction_processor_grpc_response_time",
			Help:    "The response time distribution of the GRPC Server",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "status"},
	)
)
