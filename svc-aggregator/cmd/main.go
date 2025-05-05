package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	api "github.com/etesami/detection-tracking-system/api"
	metric "github.com/etesami/detection-tracking-system/pkg/metric"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	utils "github.com/etesami/detection-tracking-system/pkg/utils"
	"github.com/etesami/detection-tracking-system/svc-aggregator/internal"
	"google.golang.org/grpc"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {

	// Setup the metric service for tracking metrics both locally and remote services
	sentDataBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BUCKETS"))
	procTimeBuckets := utils.ParseBuckets(os.Getenv("PROC_TIME_BUCKETS"))
	rttTimeBuckets := utils.ParseBuckets(os.Getenv("RTT_TIME_BUCKETS"))
	m := &metric.Metric{}
	m.RegisterMetrics(sentDataBuckets, procTimeBuckets, rttTimeBuckets)

	// Local service initialization (ingestion/aggregation) to receive a connection information
	// data sources connect to this service to inform about their address and port
	// Once the connection details are recevied, the local service will retrieve video stream over rtsp
	svcHost := os.Getenv("SVC_INGST_ADDR")
	svcPort := os.Getenv("SVC_INGST_PORT")
	if svcPort == "" || svcHost == "" {
		panic("SVC_INGST_ADDR or SVC_INGST_PORT environment variable is not set")
	}
	localSvc := &api.Service{
		Address: svcHost,
		Port:    svcPort,
	}

	// We listen on all interfaces
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", localSvc.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := &internal.Server{
		Clients:         sync.Map{},
		RegisterCh:      make(chan *api.Service, 100),
		DetectionClient: atomic.Value{},
		TrackingClient:  atomic.Value{},
	}
	grpcServer := grpc.NewServer()
	pb.RegisterDetectionTrackingPipelineServer(grpcServer, s)

	go func() {
		log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// go s.ClientHandler()

	// Setup the remote service (detection and tracking) and start the connection as client
	// when we send data to the remote services when we have to
	REMOTE_DETECTION_HOST := os.Getenv("REMOTE_DETECTION_HOST")
	REMOTE_DETECTION_PORT := os.Getenv("REMOTE_DETECTION_PORT")
	if REMOTE_DETECTION_HOST == "" || REMOTE_DETECTION_PORT == "" {
		panic("REMOTE_DETECTION_HOST or REMOTE_DETECTION_PORT environment variable is not set")
	}

	// targetDetectionSvc := api.Service{
	// 	Address: REMOTE_DETECTION_HOST,
	// 	Port:    REMOTE_DETECTION_PORT,
	// }

	// go utils.MonitorConnection(targetDetectionSvc, &s.DetectionClient)

	REMOTE_TRACKING_HOST := os.Getenv("REMOTE_TRACKING_HOST")
	REMOTE_TRACKING_PORT := os.Getenv("REMOTE_TRACKING_PORT")
	if REMOTE_TRACKING_HOST == "" || REMOTE_TRACKING_PORT == "" {
		panic("REMOTE_TRACKING_HOST or REMOTE_TRACKING_PORT environment variable is not set")
	}
	// targetTrackingSvc := api.Service{
	// 	Address: REMOTE_TRACKING_HOST,
	// 	Port:    REMOTE_TRACKING_PORT,
	// }
	// go utils.MonitorConnection(targetTrackingSvc, &s.TrackingClient)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", metricAddr, metricPort),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting metrics server on %s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Set up channel to listen for interrupt or terminate signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	// Create a context that will be cancelled on SIGINT/SIGTERM
	// ctx, cancel := context.WithCancel(context.Background())

	<-sigChan // Wait for signal
	log.Printf("Received shutdown signal\n")
	for _, vi := range s.VideoInputs {
		vi.Signal.Close()
	}
	// cancel()                  // Cancel the context
	grpcServer.GracefulStop() // Stop the gRPC server gracefully
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("Error shutting down server: %v\n", err)
	}
	log.Printf("Server shut down gracefully\n")
}
