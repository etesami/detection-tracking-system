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

	"github.com/etesami/detection-tracking-system/svc-tracker/internal"
	"google.golang.org/grpc"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {

	// Setup the metric service for tracking metrics both locally and remote services
	e2eTimeBuckets := utils.ParseBuckets(os.Getenv("E2E_TIME_BUCKETS"))
	procTimeBuckets := utils.ParseBuckets(os.Getenv("PROC_TIME_BUCKETS"))
	sentDataByteBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BYTE_BUCKETS"))
	transTimeBuckets := utils.ParseBuckets(os.Getenv("TRANSMIT_TIME_BUCKETS"))
	m := &metric.Metric{}
	m.RegisterMetrics(sentDataByteBuckets, procTimeBuckets, transTimeBuckets, e2eTimeBuckets)

	// Local service initialization (detector) to receive frames
	svcHost := os.Getenv("SVC_TRACKER_HOST")
	svcPort := os.Getenv("SVC_TRACKER_PORT")
	if svcPort == "" || svcHost == "" {
		panic("SVC_TRACKER_ADDR or SVC_TRACKER_PORT environment variable is not set")
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
	saveImageFrqTr, _ := strconv.Atoi(os.Getenv("SAVE_IMAGE_FREQUENCY_TRACKING"))
	saveImageFrqDt, _ := strconv.Atoi(os.Getenv("SAVE_IMAGE_FREQUENCY_DETECTION"))

	s := &internal.Server{
		DtConfig: &internal.DtConfig{
			Model:                os.Getenv("YOLO_MODEL"),
			ImageWidth:           width,
			ImageHeight:          height,
			SaveImage:            os.Getenv("SAVE_IMAGE") == "true",
			SaveImagePath:        os.Getenv("SAVE_IMAGE_PATH"),
			SaveImageFrequencyTr: saveImageFrqTr,
			SaveImageFrequencyDt: saveImageFrqDt,
		},
		Trackers: make(map[string]*internal.TrackerClient),
		Metric:   m,
	}
	grpcServer := grpc.NewServer()
	pb.RegisterDetectionTrackingPipelineServer(grpcServer, s)

	go func() {
		log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

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
