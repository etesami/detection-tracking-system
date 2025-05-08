package internal

import (
	"context"
	"encoding/json"
	"image"
	"log"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	mt "github.com/etesami/detection-tracking-system/pkg/metric"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	"github.com/etesami/detection-tracking-system/pkg/utils"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer
	TrackerClientRef utils.GrpcClient
	DtConfig         *DtConfig

	Metric *mt.Metric
}

type detectionData struct {
	SourceId  string
	Timestamp string
	FrameId   int64
	Boxes     []image.Rectangle
}

// YoloV8 detector model
type DtConfig struct {
	Model              string
	ImageWidth         int
	ImageHeight        int
	SaveImage          bool
	SaveImagePath      string
	SaveImageFrequency int
}

// SendFrameServer handles incoming data from ingestion/aggregation services
func (s *Server) SendFrameToServer(ctx context.Context, recData *pb.FrameData) (*pb.Ack, error) {
	recTime := time.Now()
	recTimeNano := recTime.Format(time.RFC3339Nano)
	// unmarshal metadata into a struct
	var metadata api.FrameMetadata
	if err := json.Unmarshal([]byte(recData.Metadata), &metadata); err != nil {
		log.Printf("Error unmarshalling metadata: %v", err)
		return nil, err
	}

	// process the frame data
	iboxes, indicies := s.DtConfig.ProcessFrame(recData.FrameData, int(metadata.FrameId))

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     recTimeNano,
		AckSentTimestamp:      time.Now().Format(time.RFC3339Nano),
	}

	if len(indicies) == 0 {
		log.Printf("Frame [%d]: No boxes detected.", metadata.FrameId)
		return ack, nil
	}

	selectedBoxes := make([]image.Rectangle, 0, len(indicies))
	// select only boxes with indicies
	for i := range indicies {
		if indicies[i] < 0 || indicies[i] >= len(iboxes) {
			log.Printf("[Warning] Invalid index %d for boxes", indicies[i])
			continue
		}
		selectedBoxes = append(selectedBoxes, iboxes[indicies[i]])
	}

	addProcessingTime("processing", s.Metric, recTime)

	go func(sBoxes []image.Rectangle, metadata api.FrameMetadata) {
		// construct the message for tracker service
		m := detectionData{
			SourceId:  metadata.SourceId,
			Timestamp: metadata.Timestamp,
			FrameId:   metadata.FrameId,
			Boxes:     sBoxes,
		}
		mByte, err := json.Marshal(m)
		if err != nil {
			log.Printf("Error marshalling frame data: %v", err)
			return
		}

		c := s.TrackerClientRef.Load()
		if c == nil {
			log.Println("Tracker client is not initialized")
			return
		}

		d := &pb.FrameData{
			Metadata:      string(mByte),
			FrameData:     recData.FrameData,
			SentTimestamp: time.Now().Format(time.RFC3339Nano), // the current timestamp
		}
		dByte, _ := proto.Marshal(d)
		addSentDataBytes("tracker", s.Metric, float64(len(dByte)))

		pong, err := c.SendDetectedFrameToServer(context.Background(), d)
		if err != nil {
			log.Printf("error sending frame to server: %v", err)
		}

		now := time.Now()
		transTime, err := utils.CalculateRtt(d.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, now.Format(time.RFC3339Nano))
		if err != nil {
			log.Printf("error calculating RTT: %v", err)
		}

		sTime, _ := time.Parse(time.RFC3339Nano, d.SentTimestamp)
		e2eSvcLatency := float64(now.Sub(sTime).Microseconds()) / 1000.0

		addTransitTime("tracker", s.Metric, transTime)
		addE2ELatency("tracker", s.Metric, e2eSvcLatency)

		log.Printf("Sent frame [%d] with [%d] detections, response: [%s], RTT [%.2f]ms, Total [%.2f]ms",
			int(metadata.FrameId), len(sBoxes), pong.Status, transTime, e2eSvcLatency)

	}(selectedBoxes, metadata)

	return ack, nil
}
