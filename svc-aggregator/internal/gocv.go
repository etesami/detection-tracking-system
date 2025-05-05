package internal

import (
	"context"
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	pb "github.com/etesami/detection-tracking-system/pkg/protoc"
	"github.com/etesami/detection-tracking-system/pkg/utils"

	"gocv.io/x/gocv"
)

// Config holds the configuration parameters
type Config struct {
	VideoSource        string
	QueueSize          int
	FrameRate          float64
	DetectionFrequency int
	ImageWidth         int
	ImageHeight        int
}

// VideoInput manages video ingestion and processing
type VideoInput struct {
	config      *Config
	grpcClient  *pb.DetectionTrackingPipelineClient
	queue       chan gocv.Mat
	done        chan struct{}
	frameCount  int
	emptyFrames int
	capture     *gocv.VideoCapture
	wg          sync.WaitGroup
}

// NewVideoInput creates and initializes a new VideoInput instance
func NewVideoInput(config *Config, client *pb.DetectionTrackingPipelineClient) (*VideoInput, error) {

	capture, err := gocv.OpenVideoCapture(config.VideoSource)
	if err != nil {
		return nil, fmt.Errorf("failed to open video source: %v", err)
	}

	vi := &VideoInput{
		config:      config,
		grpcClient:  client,
		queue:       make(chan gocv.Mat, config.QueueSize),
		done:        make(chan struct{}),
		capture:     capture,
		frameCount:  -1,
		emptyFrames: 0,
	}

	vi.wg.Add(1)
	go vi.processFrames()

	return vi, nil
}

// processFrames reads frames from the video source and processes them
func (vi *VideoInput) processFrames() {
	defer vi.wg.Done()

	img := gocv.NewMat()
	defer img.Close()

	// emptyFrames := 0

	for {
		select {
		case <-vi.done:
			log.Println("Stopping video input processing")
			return
		default:
			// Continue processing frames
			if ok := vi.capture.Read(&img); !ok || img.Empty() {
				log.Println("Error reading frame")
				vi.emptyFrames++
				if vi.emptyFrames > 10 {
					log.Println("Too many empty frames, stopping video input")
					vi.Close()
					return
				}
				time.Sleep(500 * time.Millisecond) // Wait before retrying
				continue
			}
			// emptyFrames = 0

			// Resize the image
			resized := gocv.NewMat()
			gocv.Resize(img, &resized, image.Pt(vi.config.ImageWidth, vi.config.ImageHeight), 0, 0, gocv.InterpolationDefault)

			select {
			case vi.queue <- resized:
				vi.frameCount++
			case <-vi.done:
				log.Println("Stopping video input processing")
				resized.Close()
				return
			}
		}
	}
}

func (vi *VideoInput) ReadFrame() error {
	select {
	case frame, ok := <-vi.queue:
		if !ok {
			return fmt.Errorf("frame queue closed")
		}
		defer frame.Close()

		log.Printf("Read frame %d from queue\n", vi.frameCount)
		buf, err := gocv.IMEncode(".jpg", frame)
		if err != nil {
			return fmt.Errorf("failed to encode frame: %v", err)
		}
		defer buf.Close()

		if err := SendFrame(buf.GetBytes(), vi.grpcClient); err != nil {
			return fmt.Errorf("failed to send frame: %v", err)
		}
	case <-vi.done:
		return fmt.Errorf("video input processing stopped")
	default:
		return fmt.Errorf("no frame available")
	}
	return nil
}

// Close cleans up resources
func (vi *VideoInput) Close() {
	close(vi.done)     // signal goroutine to stop
	vi.wg.Wait()       // wait for goroutine to finish
	vi.capture.Close() // close video source

	close(vi.queue) // close channel
	for frame := range vi.queue {
		frame.Close() // cleanup frames left in the queue
	}
	log.Println("Video input closed.")
}

func SendFrame(frame []byte, client *pb.DetectionTrackingPipelineClient) error {
	if client == nil {
		return fmt.Errorf("client is not initialized")
	}

	f := &pb.FrameData{
		FrameData:     frame,
		SentTimestamp: fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}

	pong, err := (*client).SendFrameServer(context.Background(), f)
	if err != nil {
		return fmt.Errorf("error sending frame to server: %v", err)
	}
	rtt, err := utils.CalculateRtt(f.SentTimestamp, pong.ReceivedTimestamp, pong.AckSentTimestamp, time.Now())
	if err != nil {
		return fmt.Errorf("error calculating RTT: %v", err)
	}
	log.Printf("Server response: [%s], RTT [%.2f] ms\n", pong.Status, float64(rtt)/1000.0)
	return nil
}
