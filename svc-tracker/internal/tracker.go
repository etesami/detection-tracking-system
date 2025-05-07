package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	"gocv.io/x/gocv"
)

type Server struct {
	pb.UnimplementedDetectionTrackingPipelineServer
	Trackers sync.Map
	DtConfig *DtConfig
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

// AddClient adds a new client connection data to the server
// and starts a new video input stream for that client
func (s *Server) AddDetections(sourceId string, frame []byte, detections []image.Rectangle) {
	// Check if the client already exists
	trClientIf, found := s.Trackers.Load(sourceId)

	imgMat, err := gocv.IMDecode(frame, gocv.IMReadColor)
	if err != nil {
		log.Printf("Error decoding image: %v", err)
		return
	}
	defer imgMat.Close()

	// The maximum number of detections to track: 200
	matches := make(map[int]int, 200)
	notMatched := []int{}

	if !found {
		log.Printf("Added new client, starting new boxes [%s]\n", sourceId)
		trackerInstances := make([]*TrackerInstance, 0)
		for _, bb := range detections {
			trackerInstance := NewTrackerInstance(bb)
			trackerInstance.InitTracker(imgMat)
			trackerInstances = append(trackerInstances, trackerInstance)
		}
		trClient := &TrackerClient{
			sourceId:        sourceId,
			trackerInstance: trackerInstances,
		}
		s.Trackers.Store(sourceId, trClient)
		log.Printf("Tracker added with %d boxes: [%s]\n", len(trClient.trackerInstance), sourceId)

	} else {
		log.Printf("[%s] Tracker already exists, updating boxes...\n", sourceId)
		trackerClient, ok := trClientIf.(*TrackerClient)
		if !ok {
			log.Printf("Error: unable to cast tracker to expected type\n")
			return
		}

		// Iterate over each detection
		for i, bb := range detections {
			iou := 0.0
			for i2, bb2 := range trackerClient.trackerInstance {
				iou2 := getIoU(bb, bb2.store)
				// If IoU exceeds threshold and is the highest so far, register match
				if iou2 > 0.5 && iou < iou2 {
					iou = iou2
					matches[i2] = i
				}
			}
			// If detection didn't match with any object, add index to unmatched list
			if iou == 0.0 {
				notMatched = append(notMatched, i)
			}
		}

		// If the object is already being tracked, update its position
		for i2, i := range matches {
			trackerClient.DeleteInstanceAt(i2)

			newTrackerInstance := NewTrackerInstance(detections[i])
			newTrackerInstance.InitTracker(imgMat)

			trackerClient.AddInstance(newTrackerInstance)
		}

		// If the object is not being tracked, add a new tracker instance
		for _, i := range notMatched {
			newTrackerInstance := NewTrackerInstance(detections[i])
			newTrackerInstance.InitTracker(imgMat)

			trackerClient.AddInstance(newTrackerInstance)
		}
	}

}

// RemoveClient removes a client connection data from the server
func (s *Server) RemoveClient(sourceId string) {
	if _, exists := s.Trackers.Load(sourceId); exists {
		s.Trackers.Delete(sourceId)
		log.Printf("[%s] Removed.\n", sourceId)
	} else {
		log.Panicf("[%s] Client not found.\n", sourceId)
	}
}

func (s *Server) TrackObjects(frame []byte, metadata *api.FrameMetadata) {
	// We need to look for the sourceId in within tracker clients
	// get the tracker instances and then update the tracker instances

	imgMat, err := gocv.IMDecode(frame, gocv.IMReadColor)
	if err != nil {
		log.Printf("[%d] Error decoding image: %v\n", metadata.FrameId, err)
		return
	}
	defer imgMat.Close()

	log.Printf("Tracking objects, image [%d] decoded, len: [%d] bytes...\n", metadata.FrameId, len(frame))

	trClientIf, found := s.Trackers.Load(metadata.SourceId)
	if !found {
		log.Printf("Tracking for [%d] client [%s] not found.\n", metadata.FrameId, metadata.SourceId)
		return
	}
	trackerClient, ok := trClientIf.(*TrackerClient)
	if !ok {
		log.Printf("Error: unable to cast tracker to expected type\n")
		return
	}
	log.Printf("Updating tracker for [%d] client [%s]...\n", metadata.FrameId, metadata.SourceId)
	// Iterate over each detection
	for _, trInstance := range trackerClient.trackerInstance {
		ok, err := trInstance.UpdateTracker(imgMat)
		if err != nil {
			log.Printf("Error updating tracker [%d]: %v", metadata.FrameId, err)
			continue
		}
		if !ok {
			log.Printf("Tracker update [%d] failed (disappeared?), deleting instance...\n", metadata.FrameId)
			trInstance.DeleteInstance()
			continue
		} else {
			log.Printf("Tracker updated [%d] successfully.\n", metadata.FrameId)
		}
		if s.DtConfig.SaveImage && metadata.FrameId%int64(s.DtConfig.SaveImageFrequency) == 0 {
			timestamp := time.Now().UnixNano()
			filename := fmt.Sprintf("%s/%d_track.jpg", s.DtConfig.SaveImagePath, timestamp)
			gocv.Rectangle(&imgMat, trInstance.store, color.RGBA{0, 255, 0, 0}, 2)
			if ok := gocv.IMWrite(filename, imgMat); !ok {
				log.Printf("Failed to write frame to file")
			}
		}
	}

}

// SendFrameToServer handles incoming data from ingestion/aggregation services
func (s *Server) SendFrameToServer(ctx context.Context, recData *pb.FrameData) (*pb.Ack, error) {
	recTime := time.Now().Format(time.RFC3339Nano)

	// unmarshal metadata into a struct
	var metadata api.FrameMetadata
	if err := json.Unmarshal([]byte(recData.Metadata), &metadata); err != nil {
		log.Printf("Error unmarshalling metadata: %v", err)
		return nil, err
	}

	log.Printf("Received [Aggregator] frame ID [%d]: [%d] Bytes\n", metadata.FrameId, len(recData.FrameData))

	go s.TrackObjects(recData.FrameData, &metadata)

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

	// unmarshal metadata into a struct
	var metadata detectionData
	if err := json.Unmarshal([]byte(recData.Metadata), &metadata); err != nil {
		log.Printf("Error unmarshalling metadata: %v", err)
		return nil, err
	}

	log.Printf("Received [Detector] frame ID [%d]: [%d] Bytes\n", metadata.FrameId, len(recData.FrameData))

	// Go routine for adding/updating the detection data and managing the
	// tracker instances
	go s.AddDetections(metadata.SourceId, recData.FrameData, metadata.Boxes)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     recTime,
		AckSentTimestamp:      time.Now().Format(time.RFC3339Nano),
	}

	return ack, nil
}
