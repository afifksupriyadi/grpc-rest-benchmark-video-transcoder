package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	pb "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gen/video"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/config"
	grpchandler "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/handler/grpc"
	resthandler "github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/handler/rest"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/processor"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/worker/lib/metrics"
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

	// setup processor and service
	ffmpegProcessor := processor.NewFFmpegProcessor()
	videoSvc := service.NewVideoService(ffmpegProcessor, metricsRecorder)

	// start REST server
	go func() {
		restHandler := resthandler.NewVideoHandler(videoSvc, metricsRecorder)
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
		videoServer := grpchandler.NewVideoServer(videoSvc, metricsRecorder)
		pb.RegisterWorkerServiceServer(grpcServer, videoServer)

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
