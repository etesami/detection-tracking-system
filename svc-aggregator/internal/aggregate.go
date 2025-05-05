package internal

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer

	Clients         sync.Map // map[string]*Service
	VideoInputs     []*VideoInput
	DetectionClient atomic.Value
	TrackingClient  atomic.Value

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
			QueueSize:          100,
			FrameRate:          30,
			MaxTotalFrames:     500,
			DetectionFrequency: 20,
			ImageWidth:         640,
			ImageHeight:        360,
		}
		if vi, err := NewVideoInput(&cfg, &s.DetectionClient, &s.TrackingClient); err != nil {
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
	st := time.Now()
	recTimestamp := st.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", st.Format("2006-01-02 15:04:05"), len(recData.Payload))

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
		ReceivedTimestamp:     strconv.Itoa(int(recTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}
