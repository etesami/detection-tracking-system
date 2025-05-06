package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	"github.com/etesami/detection-tracking-system/pkg/utils"

	"gocv.io/x/gocv"
)

type signal struct {
	Done chan struct{}
	once sync.Once
}

func (s *signal) Close() {
	s.once.Do(func() {
		close(s.Done)
	})
}

// Config holds the configuration parameters
type Config struct {
	VideoSource        string
	QueueSize          int
	FrameRate          float64
	MaxTotalFrames     int
	DetectionFrequency int
	ImageWidth         int
	ImageHeight        int
}

// VideoInput manages video ingestion and processing
type VideoInput struct {
	config          *Config
	grpcDtClientRef *atomic.Value
	grpcTrClientRef *atomic.Value
	queue           chan api.FrameData // Channel for frames
	Signal          signal
	frameCount      int
	frameProcessed  int
	frameSkipped    int
	capture         *gocv.VideoCapture
	wg              sync.WaitGroup // WaitGroup to wait for goroutines to finish
}

// NewVideoInput creates and initializes a new VideoInput instance
func NewVideoInput(config *Config, dtClient, trClient *atomic.Value) (*VideoInput, error) {

	log.Printf("Initializing video input with source: %s\n", config.VideoSource)
	capture, err := gocv.OpenVideoCapture(config.VideoSource)
	if err != nil {
		return nil, fmt.Errorf("failed to open video source: %v", err)
	}
	log.Printf("Starting video input processing for source: %s\n", config.VideoSource)

	vi := &VideoInput{
		config:          config,
		grpcDtClientRef: dtClient,
		grpcTrClientRef: trClient,
		queue:           make(chan api.FrameData, config.QueueSize),
		Signal:          signal{Done: make(chan struct{})},
		capture:         capture,
		frameCount:      0,
	}

	vi.wg.Add(2) // Add 2 to the WaitGroup for readFrames and processFrames
	go vi.readFrames()
	go vi.processFrames()
	// Close the video input when done
	go vi.handleClose()

	return vi, nil
}

// readFrames reads frames from the video source and processes them
func (vi *VideoInput) readFrames() {
	defer vi.wg.Done() // Mark this goroutine as done when it finishes

	img := gocv.NewMat()
	defer img.Close()

	delay := 1.0 / vi.config.FrameRate // Calculate delay based on frame rate
	log.Printf("Frame rate: %.2f, delay: %.2f seconds\n", vi.config.FrameRate, delay)
	emptyFrames := 0

	for {
		startT := time.Now()
		select {
		case <-vi.Signal.Done:
			log.Println("Stopping video input processing")
			return
		default:
			// Continue processing frames
			if ok := vi.capture.Read(&img); !ok || img.Empty() {
				log.Println("Error reading frame")
				emptyFrames++
				if emptyFrames > 10 {
					log.Println("Too many empty frames, stopping video input")
					vi.Signal.Close() // Signal to stop processing and clean up
					return
				}
				time.Sleep(500 * time.Millisecond) // Wait before retrying
				continue
			}
			emptyFrames = 0

			// Resize the image, should not close the resized image as it is passed to the queue
			// and should be closed in the consumer processFrames
			resized := gocv.NewMat()
			gocv.Resize(img, &resized, image.Pt(vi.config.ImageWidth, vi.config.ImageHeight), 0, 0, gocv.InterpolationDefault)

			frameData := api.FrameData{
				Timestamp: time.Now(),
				SourceId:  vi.config.VideoSource,
				FrameId:   vi.capture.Get(gocv.VideoCapturePosFrames),
				Frame:     resized,
			}

			select {
			case vi.queue <- frameData:
				vi.frameCount++
			case <-vi.Signal.Done:
				log.Println("Stopping video input processing")
				// since resized is closed in the consumer processFrames
				// we need to close it here to avoid memory leak
				resized.Close()
				return
			default:
				log.Println("Frame queue is full, dropping frame")
				// If the queue is full, we can choose to drop the frame or wait
				resized.Close()
				vi.frameSkipped++
			}

			elapsed := float64(time.Since(startT).Milliseconds()) / 1000.0
			sleepDuration := delay - elapsed
			if vi.frameCount%100 == 0 {
				log.Printf("Frame [%d] processing time: %.2fs, sleep: %.2fs, skipped frames: [%d]\n", vi.frameCount, elapsed, sleepDuration, vi.frameSkipped)
			}

			if sleepDuration > 0 {
				time.Sleep(time.Duration(sleepDuration) * time.Second)
			}

			if vi.config.MaxTotalFrames > 0 && vi.frameCount >= vi.config.MaxTotalFrames {
				log.Printf("Stopping video input processing after [%d] frames\n", vi.frameCount)
				vi.Signal.Close()
				return
			}

		}
	}
}

// processFrames processes frames from the queue
func (vi *VideoInput) processFrames() {
	defer vi.wg.Done()
	for {
		select {
		case f, ok := <-vi.queue:
			if !ok {
				log.Printf("frame queue closed")
				return
			}

			// if vi.frameProcessed%200 == 0 {
			// timestamp := time.Now().UnixNano()
			// filename := fmt.Sprintf("/tmp/imgs/output_%d.jpg", timestamp)
			// if ok := gocv.IMWrite(filename, frame); !ok {
			// 	log.Printf("Failed to write frame to file")
			// }
			// log.Printf("Frame [%d] proccessed.\n", vi.frameProcessed+1)
			// }

			buf, err := gocv.IMEncode(".jpg", f.Frame)
			if err != nil {
				log.Printf("Failed to encode frame: %v", err)
				vi.frameSkipped++
				f.Frame.Close()
				continue
			}

			var (
				client  = vi.grpcTrClientRef
				service = "tracker"
			)

			// Alternate between sending to the tracker and detector
			if vi.frameCount%vi.config.DetectionFrequency == 0 {
				client = vi.grpcDtClientRef
				service = "detector"
			}

			// Send the frame to the remote service using gRPC
			if err := SendFrame(f, buf.GetBytes(), client, service); err != nil {
				log.Printf("failed to send frame: %v", err)
				vi.frameSkipped++
			} else {
				vi.frameProcessed++
			}

			buf.Close()
			f.Frame.Close() // Close the frame after processing

		case <-vi.Signal.Done:
			// Closding frame will be handled in the Close method
			log.Printf("Stopping video input processing")
			return
		}
	}
}

// handleClose waits for the done channel to be closed and then closes the video input
func (vi *VideoInput) handleClose() {
	// Wait until vi.done is closed
	<-vi.Signal.Done
	log.Printf("Stopping video input processing")
	vi.Close()
}

// Close cleans up resources
func (vi *VideoInput) Close() {
	log.Printf("Closing and cleaning up...")

	log.Printf("  Waiting for goroutines to finish...")
	vi.wg.Wait() // wait for goroutines to finish: readFrames and processFrames

	log.Printf("  Closing vi.capture...")
	vi.capture.Close() // close video source

	log.Printf("  Closing vi.queue channel...")
	close(vi.queue) // close channel so there are no more frames will be added to the queue
	for f := range vi.queue {
		f.Frame.Close() // cleanup frames left in the queue
	}

	log.Println("VideoInput closed.")
}

// SendFrame sends a frame to the tracker service
func SendFrame(f api.FrameData, frameByte []byte, clientRef *atomic.Value, dstSvcName string) error {
	clientIface := clientRef.Load()
	if clientIface == nil {
		return fmt.Errorf("client is not initialized")
	}
	client := clientIface.(pb.DetectionTrackingPipelineClient)

	meta := map[string]string{
		"SourceId":  f.SourceId,
		"FrameId":   fmt.Sprintf("%f", f.FrameId),
		"Timestamp": f.Timestamp.Format(time.RFC3339),
	}
	metaByte, _ := json.Marshal(meta)

	d := &pb.FrameData{
		FrameData:     frameByte,
		Metadata:      string(metaByte),
		SentTimestamp: time.Now().Format(time.RFC3339),
	}

	pong, err := client.SendFrameToServer(context.Background(), d)
	if err != nil {
		return fmt.Errorf("error sending frame to server: %v", err)
	}
	rtt, err := utils.CalculateRtt(d.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now())
	if err != nil {
		return fmt.Errorf("error calculating RTT: %v", err)
	}
	log.Printf("Sent frame [%f], [%s] response: [%s], RTT [%.2f] ms\n", f.FrameId, dstSvcName, pong.Status, float64(rtt)/1000.0)
	return nil
}
