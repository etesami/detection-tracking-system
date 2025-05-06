package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	"github.com/etesami/detection-tracking-system/pkg/utils"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer

	Clients     sync.Map // map[string]*Service
	VideoInputs []*VideoInput
	DtClient    atomic.Value
	TrClient    atomic.Value

	// Channel for a new client
	RegisterCh chan *api.Service
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
			QueueSize:          120,
			FrameRate:          10,
			MaxTotalFrames:     -1, // -1 means no limit
			DetectionFrequency: 20,
			ImageWidth:         640,
			ImageHeight:        360,
		}
		if vi, err := NewVideoInput(&cfg, &s.DtClient, &s.TrClient); err != nil {
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
	// recTimestamp := st.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", recTime, len(recData.Payload))

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

// SendFrame sends a frame to the tracker service
func SendFrame(f api.FrameData, frameByte []byte, clientRef *atomic.Value, dstSvcName string) error {
	clientIface := clientRef.Load()
	if clientIface == nil {
		return fmt.Errorf("client is not initialized")
	}
	client := clientIface.(pb.DetectionTrackingPipelineClient)

	meta := api.FrameData{
		SourceId:  f.SourceId,
		FrameId:   f.FrameId,
		Timestamp: f.Timestamp,
	}
	metaByte, _ := json.Marshal(meta)

	d := &pb.FrameData{
		FrameData:     frameByte,
		Metadata:      string(metaByte),
		SentTimestamp: time.Now().Format(time.RFC3339Nano),
	}

	pong, err := client.SendFrameToServer(context.Background(), d)
	if err != nil {
		return fmt.Errorf("error sending frame to server: %v", err)
	}

	rtt, err := utils.CalculateRtt(d.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("error calculating RTT: %v", err)
	}
	log.Printf("Sent frame [%d], [%s] response: [%s], RTT [%.2f] ms\n", f.FrameId, dstSvcName, pong.Status, float64(rtt)/1000.0)
	return nil
}
