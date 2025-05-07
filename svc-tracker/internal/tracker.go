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
	Trackers map[string]*TrackerClient
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
	Model                string
	ImageWidth           int
	ImageHeight          int
	SaveImage            bool
	SaveImagePath        string
	SaveImageFrequencyTr int
	SaveImageFrequencyDt int
}

// AddClient adds a new client connection data to the server
// and starts a new video input stream for that client
func (s *Server) AddDetections(sourceId string, frameId int64, frame []byte, detections []image.Rectangle) {
	sourceName := "Detect"

	imgMat, err := gocv.IMDecode(frame, gocv.IMReadColor)
	if err != nil {
		log.Printf("Frame [%d], [%s]: Error decoding image: %v", frameId, sourceName, err)
		return
	}
	defer imgMat.Close()

	// Check if the client already exists
	trClient, found := s.Trackers[sourceId]

	// The maximum number of detections to track: 200
	matches := make(map[uint64]int, 200)
	notMatched := []int{}

	if !found {
		// trackerInstances := make([]*TrackerInstance, 0)
		trClient = &TrackerClient{
			sourceId:        sourceId,
			trackerInstance: sync.Map{},
		}
		for _, bb := range detections {
			trackerInstance := NewTrackerInstance(bb)
			trackerInstance.InitTracker(imgMat)
			trClient.AddInstance(trackerInstance)
		}
		s.Trackers[sourceId] = trClient
		log.Printf("Frame [%d], [%s]: Tracker added with (%d) boxes: [%s]",
			frameId, sourceName, len(detections), sourceId)

	} else {
		log.Printf("Frame [%d], [%s]: Tracker already exists, updating boxes [%s]", frameId, sourceName, sourceId)

		// Iterate over each detection
		for i, bb := range detections {
			iou := 0.0
			trClient.trackerInstance.Range(func(key, value interface{}) bool {
				if i2, ok := key.(uint64); ok {
					if trInstance, ok := value.(*TrackerInstance); ok {
						// Calculate IoU between detection and tracker instance
						iou2 := getIoU(bb, trInstance.store)
						// If IoU exceeds threshold and is the highest so far, register match
						if iou2 > 0.5 && iou < iou2 {
							iou = iou2
							matches[i2] = i
						}
					}
				}
				return true
			})
			// If detection didn't match with any object, add index to unmatched list
			if iou == 0.0 {
				notMatched = append(notMatched, i)
			}
			// log.Printf("Frame [%d], [%s]: Detection [%d] IoU: %f", frameId, sourceName, i, iou)
		}

		log.Printf("Frame [%d], [%s]: Mathces: [%d], Unmatched: [%d]",
			frameId, sourceName, len(matches), len(notMatched))

		// If the object is already being tracked, update its position
		for i2, i := range matches {
			trClient.DeleteInstanceAt(i2)

			newTrackerInstance := NewTrackerInstance(detections[i])
			newTrackerInstance.InitTracker(imgMat)

			trClient.AddInstance(newTrackerInstance)
		}

		// If the object is not being tracked, add a new tracker instance
		for _, i := range notMatched {
			newTrackerInstance := NewTrackerInstance(detections[i])
			newTrackerInstance.InitTracker(imgMat)

			trClient.AddInstance(newTrackerInstance)
		}
	}

	if s.DtConfig.SaveImage && frameId%int64(s.DtConfig.SaveImageFrequencyDt) == 0 {
		timestamp := time.Now().UnixNano()
		filename := fmt.Sprintf("%s/%d_f%d_detect.jpg", s.DtConfig.SaveImagePath, timestamp, frameId)
		recs := make([]image.Rectangle, 0)
		trClient.trackerInstance.Range(func(key, value interface{}) bool {
			if _, ok := key.(uint64); ok {
				if trInstance, ok := value.(*TrackerInstance); ok {
					recs = append(recs, trInstance.store)
				}
			}
			return true
		})
		log.Printf("Frame [%d], [%s]: Saving image as %d_f%d_detect.jpg", frameId, sourceName, timestamp, frameId)
		for _, rc := range recs {
			gocv.Rectangle(&imgMat, rc, color.RGBA{0, 255, 0, 0}, 2)
		}
		if ok := gocv.IMWrite(filename, imgMat); !ok {
			log.Printf("Frame [%d], [%s]: Failed to write frame to file", frameId, sourceName)
		}
	}
}

func (s *Server) TrackObjects(frame []byte, metadata *api.FrameMetadata) {
	sourceName := "Track"

	imgMat, err := gocv.IMDecode(frame, gocv.IMReadColor)
	if err != nil {
		log.Printf("Frame [%d], [%s]: Error decoding image: %v", metadata.FrameId, sourceName, err)
		return
	}
	defer imgMat.Close()

	trClient, found := s.Trackers[metadata.SourceId]
	if !found {
		log.Printf("Frame [%d], [%s]: Tracking not found.", metadata.FrameId, sourceName)
		return
	}

	lostInstances := make([]uint64, 0)
	totalDetections := 0
	trClient.trackerInstance.Range(func(key, value interface{}) bool {
		if i, ok := key.(uint64); ok {
			if trInstance, ok := value.(*TrackerInstance); ok {
				if ok := trInstance.UpdateTracker(imgMat); !ok {
					lostInstances = append(lostInstances, i)
				}
				totalDetections++
			}
		}
		return true // continue iteration
	})

	log.Printf("Frame [%d], [%s]: Lost trackings: [%d]/ [%d]",
		metadata.FrameId, sourceName, len(lostInstances), totalDetections)

	// Delete lost instances
	deleted := 0
	for _, i := range lostInstances {
		trClient.DeleteInstanceAt(i)
		deleted++
	}
	log.Printf("Frame [%d], [%s]: Deleted [%d] lost trackings.", metadata.FrameId, sourceName, deleted)

	if s.DtConfig.SaveImage && metadata.FrameId%int64(s.DtConfig.SaveImageFrequencyTr) == 0 {
		timestamp := time.Now().UnixNano()
		filename := fmt.Sprintf("%s/%d_f%d_track.jpg", s.DtConfig.SaveImagePath, timestamp, metadata.FrameId)
		// since drawing takes time, we first get all instances and then use them to draw
		recs := make([]image.Rectangle, 0)
		trClient.trackerInstance.Range(func(key, value interface{}) bool {
			if _, ok := key.(uint64); ok {
				if trInstance, ok := value.(*TrackerInstance); ok {
					recs = append(recs, trInstance.store)
				}
			}
			return true
		})
		log.Printf("Frame [%d], [%s]: Saving image as %d_f%d_track.jpg", metadata.FrameId, sourceName, timestamp, metadata.FrameId)
		for _, rc := range recs {
			// Draw the bounding box
			gocv.Rectangle(&imgMat, rc, color.RGBA{0, 255, 0, 0}, 2)
		}
		if ok := gocv.IMWrite(filename, imgMat); !ok {
			log.Printf("Frame [%d], [%s]: Failed to write frame to file", metadata.FrameId, sourceName)
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

	// log.Printf("Frame [%d], [%s]: Received: [%d] Bytes\n", metadata.FrameId, "Track", len(recData.FrameData))

	s.TrackObjects(recData.FrameData, &metadata)

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

	// log.Printf("Frame [%d], [%s]: Received: [%d] Bytes, Detections: [%d]", metadata.FrameId, "Detect", len(recData.FrameData), len(metadata.Boxes))

	// Go routine for adding/updating the detection data and managing the
	// tracker instances
	s.AddDetections(metadata.SourceId, metadata.FrameId, recData.FrameData, metadata.Boxes)

	ack := &pb.Ack{
		Status:                "ok",
		OriginalSentTimestamp: recData.SentTimestamp,
		ReceivedTimestamp:     recTime,
		AckSentTimestamp:      time.Now().Format(time.RFC3339Nano),
	}

	return ack, nil
}
