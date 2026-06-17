package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/config"
	grpchandler "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/handler/grpc"
	resthandler "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/handler/rest"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	workergrpc "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/worker/grpc"
	workerrest "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/worker/rest"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/httpclient"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/metrics"
	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Get()

	// setup metrics
	registry := prometheus.NewRegistry()
	metricsRecorder := metrics.NewPrometheusRecorder(registry)

	// setup HTTP client for REST worker communication
	httpClient := httpclient.NewRestyClient(cfg.HTTPClient.Timeout)

	// setup worker clients
	restWorkerClient := workerrest.NewRestWorkerClient(httpClient, cfg.Worker.Worker1RESTAddr)
	grpcWorkerClient, err := workergrpc.NewGrpcWorkerClient(cfg.Worker.Worker1GRPCAddr)
	if err != nil {
		log.Fatalf("failed to connect to grpc worker: %v", err)
	}

	// setup services
	restVideoSvc := service.NewVideoService(restWorkerClient, metricsRecorder)
	grpcVideoSvc := service.NewVideoService(grpcWorkerClient, metricsRecorder)

	// start REST server
	go func() {
		restHandler := resthandler.NewVideoHandler(restVideoSvc, metricsRecorder)
		r := gin.Default()
		resthandler.RegisterRoutes(r, restHandler)
		addr := fmt.Sprintf(":%d", cfg.RESTPort)
		log.Printf("REST server listening on %s", addr)
		if err := r.Run(addr); err != nil {
			log.Fatalf("REST server failed: %v", err)
		}
	}()

	// start gRPC server
	go func() {
		grpcServer := grpc.NewServer()
		videoServer := grpchandler.NewVideoServer(grpcVideoSvc, metricsRecorder)
		pb.RegisterGatewayServiceServer(grpcServer, videoServer)

		addr := fmt.Sprintf(":%d", cfg.GRPCPort)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen on %s: %v", addr, err)
		}
		log.Printf("gRPC server listening on %s", addr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// start metrics server
	metricsAddr := fmt.Sprintf(":%d", cfg.MetricsPort)
	log.Printf("metrics server listening on %s", metricsAddr)
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	if err := http.ListenAndServe(metricsAddr, nil); err != nil {
		log.Fatalf("metrics server failed: %v", err)
	}
}
