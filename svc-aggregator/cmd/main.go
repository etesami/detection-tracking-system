package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

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
		Clients: sync.Map{},
	}
	grpcServer := grpc.NewServer()
	pb.RegisterDetectionTrackingPipelineServer(grpcServer, s)

	go func() {
		log.Printf("starting gRPC server on port %s:%s\n", localSvc.Address, localSvc.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	go s.ClientHandler()

	// // Setup the remote service (detection and tracking) and start the connection as client
	// // when we send data to the remote services when we have to
	// REMOTE_SVC_HOST := os.Getenv("REMOTE_SVC_HOST")
	// REMOTE_SVC_PORT := os.Getenv("REMOTE_SVC_PORT")
	// if REMOTE_SVC_HOST == "" || REMOTE_SVC_PORT == "" {
	// 	panic("REMOTE_SVC_HOST or REMOTE_SVC_PORT environment variable is not set")
	// }

	// targetSvc := &api.Service{
	// 	Address: REMOTE_SVC_HOST,
	// 	Port:    REMOTE_SVC_PORT,
	// }

	// var conn *grpc.ClientConn
	// var client pb.DetectionTrackingPipelineClient
	// go func() {
	// 	for {
	// 		if err := targetSvc.ServiceReachable(); err == nil {
	// 			var err error
	// 			conn, err = grpc.NewClient(
	// 				targetSvc.Address+":"+targetSvc.Port,
	// 				grpc.WithTransportCredentials(insecure.NewCredentials()))
	// 			if err != nil {
	// 				log.Printf("Failed to connect to target service: %v", err)
	// 				return
	// 			}
	// 			client = pb.NewDetectionTrackingPipelineClient(conn)
	// 			log.Printf("Connected to target service: %s:%s\n", targetSvc.Address, targetSvc.Port)
	// 			return
	// 		} else {
	// 			log.Printf("Target service is not reachable: %v", err)
	// 			time.Sleep(5 * time.Second)
	// 		}
	// 	}
	// }()
	// defer conn.Close()

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}
