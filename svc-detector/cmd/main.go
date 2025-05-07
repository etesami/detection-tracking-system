package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	api "github.com/etesami/detection-tracking-system/api"
	metric "github.com/etesami/detection-tracking-system/pkg/metric"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	utils "github.com/etesami/detection-tracking-system/pkg/utils"

	"github.com/etesami/detection-tracking-system/svc-detector/internal"
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

	// Local service initialization (detector) to receive frames
	svcHost := os.Getenv("SVC_DETECTOR_HOST")
	svcPort := os.Getenv("SVC_DETECTOR_PORT")
	if svcPort == "" || svcHost == "" {
		panic("SVC_DETECTOR_ADDR or SVC_DETECTOR_PORT environment variable is not set")
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

	width, _ := strconv.Atoi(os.Getenv("IMAGE_WIDTH"))
	height, _ := strconv.Atoi(os.Getenv("IMAGE_HEIGHT"))
	saveImageFrq, _ := strconv.Atoi(os.Getenv("SAVE_IMAGE_FREQUENCY"))

	s := &internal.Server{
		TrackerClientRef: utils.GrpcClient{},
		DtConfig: &internal.DtConfig{
			Model:              os.Getenv("YOLO_MODEL"),
			ImageWidth:         width,
			ImageHeight:        height,
			SaveImage:          os.Getenv("SAVE_IMAGE") == "true",
			SaveImagePath:      os.Getenv("SAVE_IMAGE_PATH"),
			SaveImageFrequency: saveImageFrq,
		},
	}
	grpcServer := grpc.NewServer()
	pb.RegisterDetectionTrackingPipelineServer(grpcServer, s)

	go func() {
		log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Setup the remote service (tracker) to send processed frames
	REMOTE_TRACKER_HOST := os.Getenv("REMOTE_TRACKER_HOST")
	REMOTE_TRACKER_PORT := os.Getenv("REMOTE_TRACKER_PORT")
	if REMOTE_TRACKER_HOST == "" || REMOTE_TRACKER_PORT == "" {
		panic("REMOTE_TRACKER_HOST or REMOTE_TRACKER_PORT environment variable is not set")
	}

	targetSvc := api.Service{
		Address: REMOTE_TRACKER_HOST,
		Port:    REMOTE_TRACKER_PORT,
	}

	go utils.MonitorConnection1(targetSvc, &s.TrackerClientRef)

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
	// cancel()                  // Cancel the context
	grpcServer.GracefulStop() // Stop the gRPC server gracefully
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("Error shutting down server: %v\n", err)
	}
	log.Printf("Server shut down gracefully\n")
}
