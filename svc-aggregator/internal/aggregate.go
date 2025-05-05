package internal

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer
	// Client *pb.DetectionTrackingPipelineClient
	Clients sync.Map // map[string]*Service
	// Channel for a new client
	registerCh chan *api.Service
}

func (s *Server) AddClient(address, port string) {
	c := api.Service{
		Address: address,
		Port:    port,
	}
	_, loaded := s.Clients.LoadOrStore(fmt.Sprintf("%s:%s", address, port), &c)
	if !loaded {
		s.registerCh <- &c
		log.Printf("Added new client: %s:%s\n", address, port)
	}
}

func (s *Server) RemoveClient(address string) {
	if _, exists := s.Clients.Load(address); exists {
		s.Clients.Delete(address)
		log.Printf("Removed client: %s\n", address)
	} else {
		log.Printf("Client not found: %s\n", address)
	}
}

func (s *Server) SendDataToServer(ctx context.Context, recData *pb.Data) (*pb.Ack, error) {
	st := time.Now()
	recTimestamp := st.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", st.Format("2006-01-02 15:04:05"), len(recData.Payload))

	// Expect the message to be in the format of host:ip
	parts := strings.Split(recData.Payload, ":")
	if len(parts) != 2 {
		log.Printf("Invalid payload format: %s", recData.Payload)
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

func (s *Server) ClientHandler() {
	for svc := range s.registerCh {
		fmt.Println("New client registered:", svc.Address)
	}
}
