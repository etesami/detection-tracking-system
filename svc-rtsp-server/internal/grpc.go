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
func ProcessTicker(clientRef *utils.GrpcClient, serverName string, metricList *metric.Metric, rtspPort string) error {

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
		rtt, err := utils.CalculateRtt(ping.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now().Format(time.RFC3339Nano))
		if err != nil {
			log.Printf("Error calculating RTT: %v", err)
			return
		}
		m.AddRttTime(serverName, float64(rtt)/1000.0)
		log.Printf("Sever response: [%s], RTT [%.2f] ms\n", pong.Status, float64(rtt)/1000.0)
	}(metricList)

	return nil
}
