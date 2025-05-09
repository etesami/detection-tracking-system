package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	metric "github.com/etesami/detection-tracking-system/pkg/metric"
	utils "github.com/etesami/detection-tracking-system/pkg/utils"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/etesami/detection-tracking-system/svc-rtsp-server/internal"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {

	e2eTimeBuckets := utils.ParseBuckets(os.Getenv("E2E_TIME_BUCKETS"))
	procTimeBuckets := utils.ParseBuckets(os.Getenv("PROC_TIME_BUCKETS"))
	sentDataByteBuckets := utils.ParseBuckets(os.Getenv("SENT_DATA_BYTE_BUCKETS"))
	transTimeBuckets := utils.ParseBuckets(os.Getenv("TRANSMIT_TIME_BUCKETS"))
	m := &metric.Metric{}
	m.RegisterMetrics(sentDataByteBuckets, procTimeBuckets, transTimeBuckets, e2eTimeBuckets)

	updateFrequencyStr := os.Getenv("UPDATE_FREQUENCY")
	if updateFrequencyStr == "" {
		updateFrequencyStr = "5" // Default to 5 seconds if not set
	}
	updateFrequency, err := strconv.Atoi(updateFrequencyStr)
	if err != nil {
		log.Fatalf("Error parsing update frequency: %v", err)
	}

	// Local rtsp server initialization
	RTSP_SERVER_HOST := os.Getenv("RTSP_SERVER_HOST")
	RTSP_SERVER_PORT := os.Getenv("RTSP_SERVER_PORT")
	if RTSP_SERVER_HOST == "" || RTSP_SERVER_PORT == "" {
		panic("RTSP_SERVER_HOST or RTSP_SERVER_POR   T environment variable is not set")
	}

	FILEPATH := os.Getenv("FILEPATH")
	if FILEPATH == "" {
		panic("FILEPATH environment variable is not set")
	}
	localSvc := api.Service{
		Address: RTSP_SERVER_HOST,
		Port:    RTSP_SERVER_PORT,
	}

	go startRTSPStream(localSvc, FILEPATH)

	// Remote service initialization (aggregator)
	REMOTE_SVC_HOST := os.Getenv("REMOTE_SVC_HOST")
	REMOTE_SVC_PORT := os.Getenv("REMOTE_SVC_PORT")
	if REMOTE_SVC_HOST == "" || REMOTE_SVC_PORT == "" {
		panic("REMOTE_SVC_HOST or REMOTE_SVC_PORT environment variable is not set")
	}
	targetSvc := api.Service{
		Address: REMOTE_SVC_HOST,
		Port:    REMOTE_SVC_PORT,
	}
	var client utils.GrpcClient
	go utils.MonitorConnection1(targetSvc, &client)

	// First call to processTicker
	time.Sleep(2 * time.Second) // Wait a few seconds before the first call to let connection be established
	if err := internal.ProcessTicker(&client, "aggregator", m, RTSP_SERVER_PORT); err != nil {
		log.Printf("Error during processing: %v", err)
	}

	// Set up a ticker to periodically call the gRPC server to measure the RTT
	ticker := time.NewTicker(time.Duration(updateFrequency) * time.Second)
	defer ticker.Stop()

	log.Printf("Update frequency: %d seconds\n", updateFrequency)
	go func(m *metric.Metric, c *utils.GrpcClient) {
		for range ticker.C {
			if err := internal.ProcessTicker(c, "aggregator", m, RTSP_SERVER_PORT); err != nil {
				log.Printf("Error during processing: %v", err)
			}
		}
	}(m, &client)

	metricAddr := os.Getenv("METRIC_ADDR")
	metricPort := os.Getenv("METRIC_PORT")
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting server on :%s\n", metricPort)
	http.ListenAndServe(fmt.Sprintf("%s:%s", metricAddr, metricPort), nil)
}

// startRTSPStream starts an RTSP server that streams a file in MPEG-TS format.
func startRTSPStream(t api.Service, filePath string) {
	h := &internal.ServerHandler{}

	// prevent clients from connecting to the server until the stream is properly set up
	h.Mutex.Lock()

	// create the server
	h.Server = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       fmt.Sprintf("%s:%s", t.Address, t.Port),
		UDPRTPAddress:     fmt.Sprintf("%s:8000", t.Address),
		UDPRTCPAddress:    fmt.Sprintf("%s:8001", t.Address),
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}

	// start the server
	err := h.Server.Start()
	if err != nil {
		panic(err)
	}
	defer h.Server.Close()

	// create a RTSP description that contains a H264 format
	desc := &description.Session{
		Medias: []*description.Media{{
			Type: description.MediaTypeVideo,
			Formats: []format.Format{&format.H264{
				PayloadTyp:        96,
				PacketizationMode: 1,
			}},
		}},
	}

	// create a server stream
	h.Stream = &gortsplib.ServerStream{
		Server: h.Server,
		Desc:   desc,
	}
	err = h.Stream.Initialize()
	if err != nil {
		panic(err)
	}
	defer h.Stream.Close()

	// open a file in MPEG-TS format
	f, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// in a separate routine, route frames from file to ServerStream
	go internal.RouteFrames(f, h.Stream)

	// allow clients to connect
	h.Mutex.Unlock()

	// wait until a fatal error
	log.Printf("server is ready on %s", h.Server.RTSPAddress)
	panic(h.Server.Wait())
}
