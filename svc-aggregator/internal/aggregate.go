package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	mt "github.com/etesami/detection-tracking-system/pkg/metric"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	"github.com/etesami/detection-tracking-system/pkg/utils"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer

	Clients     sync.Map // map[string]*Service
	VideoInputs []*VideoInput
	DtClient    utils.GrpcClient
	TrClient    utils.GrpcClient

	// Channel for a new client
	RegisterCh   chan *api.Service
	GlovalConfig *Config

	Metric *mt.Metric
}

// AddClient adds a new client connection data to the server
// and starts a new video input stream for that client
func (s *Server) AddClient(address, port string) {
	c := api.Service{
		Address: address,
		Port:    port,
	}
	// Check if the client already exists
	_, loaded := s.Clients.LoadOrStore(fmt.Sprintf("%s:%s", address, port), &c)

	if !loaded {
		log.Printf("Added new client: %s:%s\n", address, port)
		cfg := Config{
			VideoSource:        fmt.Sprintf("rtsp://%s:%s/stream", address, port),
			QueueSize:          s.GlovalConfig.QueueSize,
			FrameRate:          float64(s.GlovalConfig.FrameRate),
			MaxTotalFrames:     s.GlovalConfig.MaxTotalFrames,
			DetectionFrequency: s.GlovalConfig.DetectionFrequency,
			ImageWidth:         640,
			ImageHeight:        360,
		}
		if vi, err := NewVideoInput(&cfg, &s.DtClient, &s.TrClient, s.Metric); err != nil {
			log.Printf("Error creating video input: %v\n", err)
			return
		} else {
			s.VideoInputs = append(s.VideoInputs, vi)
		}

		log.Printf("Video input created for client: %s:%s\n", address, port)
	}
}

// RemoveClient removes a client connection data from the server
func (s *Server) RemoveClient(address, port string) {
	if _, exists := s.Clients.Load(fmt.Sprintf("%s:%s", address, port)); exists {
		s.Clients.Delete(fmt.Sprintf("%s:%s", address, port))
		log.Printf("Removed client: %s:%s\n", address, port)
	} else {
		log.Printf("Client not found: %s:%s\n", address, port)
	}
}

// SendDataToServer handles incoming data from clients
func (s *Server) SendDataToServer(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	recTime := time.Now().Format(time.RFC3339Nano)
	// log.Printf("Received at [%s]: [%d] Bytes\n", recTime, len(recData.Payload))

	// Expect the message to be in the format of host:ip
	parts := strings.Split(recData.Payload, ":")
	if len(parts) != 2 {
		log.Printf("Invalid payload format: %s", recData.Payload)
		return nil, fmt.Errorf("invalid payload format")
	} else {
		// add connection information to the list of clients if not already present
		s.AddClient(parts[0], parts[1])
	}

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     recTime,
		AckSentTimestamp:      time.Now().Format(time.RFC3339Nano),
	}

	return ack, nil
}

// SendFrame sends a frame to the detector/tracker service
func SendFrame(f api.FrameMetadata, frameByte []byte, clientRef *utils.GrpcClient, m *mt.Metric, dstSvcName string) error {
	client := clientRef.Load()
	if client == nil {
		return fmt.Errorf("client is not initialized")
	}

	metaByte, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("error marshalling metadata: %v", err)
	}

	d := &pb.FrameData{
		FrameData:     frameByte,
		Metadata:      string(metaByte),
		SentTimestamp: time.Now().Format(time.RFC3339Nano),
	}

	pong, err := client.SendFrameToServer(context.Background(), d)
	if err != nil {
		return fmt.Errorf("error sending frame to server: %v", err)
	}
	dByte, _ := proto.Marshal(d)
	addSentDataBytes(dstSvcName, m, float64(len(dByte)))

	now := time.Now()
	transTime, err := utils.CalculateRtt(d.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("error calculating RTT: %v", err)
	}

	sTime, _ := time.Parse(time.RFC3339Nano, d.SentTimestamp)
	e2eSvcLatency := float64(now.Sub(sTime).Microseconds()) / 1000.0

	addTransitTime(dstSvcName, m, transTime)
	addE2ELatency(dstSvcName, m, e2eSvcLatency)

	log.Printf("Sent frame [%d], [%s] response: [%s], RTT [%.2f]ms, Total [%.2f]ms", f.FrameId, dstSvcName, pong.Status, transTime, e2eSvcLatency)
	return nil
}
