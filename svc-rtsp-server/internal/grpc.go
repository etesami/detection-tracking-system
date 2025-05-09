package internal

import (
	"context"
	"fmt"
	"log"
	"time"

	metric "github.com/etesami/detection-tracking-system/pkg/metric"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	utils "github.com/etesami/detection-tracking-system/pkg/utils"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer
	Client *pb.DetectionTrackingPipelineClient
	Metric *metric.Metric
}

// processTicker processes the ticker event
func ProcessTicker(clientRef *utils.GrpcClient, serverName string, m *metric.Metric, rtspPort string) error {

	client := clientRef.Load()
	if client == nil {
		return nil
	}

	ip, err := utils.GetOutboundIP()
	if err != nil {
		log.Printf("Error getting outbound IP: %v", err)
	}
	go func(m *metric.Metric) {
		ping := &pb.Data{
			Payload:       fmt.Sprintf("%s:%s", ip, rtspPort),
			SentTimestamp: time.Now().Format(time.RFC3339Nano),
		}
		pong, err := client.SendDataToServer(context.Background(), ping)
		// in case the target service is not reachable anymore we should just return
		if err != nil {
			log.Printf("Error sending data to server: %v", err)
			return
		}

		now := time.Now()
		transTime, err := utils.CalculateRtt(ping.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, now.Format(time.RFC3339Nano))
		if err != nil {
			log.Printf("error calculating RTT: %v", err)
		}

		sTime, _ := time.Parse(time.RFC3339Nano, ping.SentTimestamp)
		e2eSvcLatency := float64(now.Sub(sTime).Microseconds()) / 1000.0

		addTransitTime("aggregator", m, transTime)
		addE2ELatency("aggregator", m, e2eSvcLatency)

		log.Printf("Sever response: [%s], Transit [%.2f]ms, Total: [%.2f]ms", pong.Status, transTime, e2eSvcLatency)
	}(m)

	return nil
}
