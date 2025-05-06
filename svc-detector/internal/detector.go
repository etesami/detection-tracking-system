package internal

import (
	"context"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer
	TrackerClientRef atomic.Value
}

// SendFrameServer handles incoming data from ingestion/aggregation services
func (s *Server) SendFrameServer(ctx context.Context, recData *pb.FrameData) (*pb.Ack, error) {
	st := time.Now()
	recTimestamp := st.UnixMilli()
	log.Printf("Received at [%s]: [%d]\n", st.Format("2006-01-02 15:04:05"), len(recData.FrameData))

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     strconv.Itoa(int(recTimestamp)),
		AckSentTimestamp:      strconv.Itoa(int(time.Now().UnixMilli())),
	}

	return ack, nil
}
