package internal

import (
	"context"
	"encoding/json"
	"image"
	"log"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer
	DtConfig *DtConfig
}

type frameWithBoxes struct {
	SourceId  string
	Timestamp string
	FrameId   int64
	Boxes     []image.Rectangle
	Indicies  []int
	Frame     []byte
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
	recTime := time.Now().Format(time.RFC3339Nano)
	log.Printf("Received [Aggregator] at [%s]: [%d]\n", recTime, len(recData.FrameData))

	// unmarshal metadata into a struct
	var metadata api.FrameData
	if err := json.Unmarshal([]byte(recData.Metadata), &metadata); err != nil {
		log.Printf("Error unmarshalling metadata: %v", err)
		return nil, err
	}

	// process the frame data
	// iboxes, indicies := s.DtConfig.ProcessFrame(recData.FrameData, int(metadata.FrameId))

	// go func() {
	// 	// construct the message for tracker service
	// 	m := frameWithBoxes{
	// 		SourceId:  metadata.SourceId,
	// 		Timestamp: metadata.Timestamp,
	// 		FrameId:   metadata.FrameId,
	// 		Boxes:     iboxes,
	// 		Indicies:  indicies,
	// 	}
	// 	mByte, err := json.Marshal(m)
	// 	if err != nil {
	// 		log.Printf("Error marshalling frame data: %v", err)
	// 		return
	// 	}

	// 	clientIface := s.TrackerClientRef.Load()
	// 	if clientIface == nil {
	// 		log.Println("Tracker client is not initialized")
	// 		return
	// 	}

	// 	c := clientIface.(pb.DetectionTrackingPipelineClient)
	// 	d := pb.FrameData{
	// 		Metadata:      string(mByte),
	// 		FrameData:     recData.FrameData,
	// 		SentTimestamp: time.Now().Format(time.RFC3339Nano), // the current timestamp
	// 	}
	// 	pong, err := c.SendFrameToServer(context.Background(), &d)
	// 	if err != nil {
	// 		log.Printf("error sending frame to server: %v", err)
	// 	}

	// 	rtt, err := utils.CalculateRtt(d.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now().Format(time.RFC3339Nano))
	// 	if err != nil {
	// 		log.Printf("error calculating RTT: %v", err)
	// 	}
	// 	log.Printf("Sent frame [%d], response: [%s], RTT [%.2f] ms\n", int(metadata.FrameId), pong.Status, float64(rtt)/1000.0)

	// }()

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     recTime,
		AckSentTimestamp:      time.Now().Format(time.RFC3339Nano),
	}

	return ack, nil
}

// SendFrameServer handles incoming data from detector service
func (s *Server) SendDetectedFrameToServer(ctx context.Context, recData *pb.FrameData) (*pb.Ack, error) {
	recTime := time.Now().Format(time.RFC3339Nano)
	log.Printf("Received [Detector] at [%s]: [%d]\n", recTime, len(recData.FrameData))

	// unmarshal metadata into a struct
	var metadata api.FrameData
	if err := json.Unmarshal([]byte(recData.Metadata), &metadata); err != nil {
		log.Printf("Error unmarshalling metadata: %v", err)
		return nil, err
	}

	// process the frame data
	// iboxes, indicies := s.DtConfig.ProcessFrame(recData.FrameData, int(metadata.FrameId))

	// go func() {
	// 	// construct the message for tracker service
	// 	m := frameWithBoxes{
	// 		SourceId:  metadata.SourceId,
	// 		Timestamp: metadata.Timestamp,
	// 		FrameId:   metadata.FrameId,
	// 		Boxes:     iboxes,
	// 		Indicies:  indicies,
	// 	}
	// 	mByte, err := json.Marshal(m)
	// 	if err != nil {
	// 		log.Printf("Error marshalling frame data: %v", err)
	// 		return
	// 	}

	// 	clientIface := s.TrackerClientRef.Load()
	// 	if clientIface == nil {
	// 		log.Println("Tracker client is not initialized")
	// 		return
	// 	}

	// 	c := clientIface.(pb.DetectionTrackingPipelineClient)
	// 	d := pb.FrameData{
	// 		Metadata:      string(mByte),
	// 		FrameData:     recData.FrameData,
	// 		SentTimestamp: time.Now().Format(time.RFC3339Nano), // the current timestamp
	// 	}
	// 	pong, err := c.SendFrameToServer(context.Background(), &d)
	// 	if err != nil {
	// 		log.Printf("error sending frame to server: %v", err)
	// 	}

	// 	rtt, err := utils.CalculateRtt(d.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now().Format(time.RFC3339Nano))
	// 	if err != nil {
	// 		log.Printf("error calculating RTT: %v", err)
	// 	}
	// 	log.Printf("Sent frame [%d], response: [%s], RTT [%.2f] ms\n", int(metadata.FrameId), pong.Status, float64(rtt)/1000.0)

	// }()

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     recTime,
		AckSentTimestamp:      time.Now().Format(time.RFC3339Nano),
	}

	return ack, nil
}
