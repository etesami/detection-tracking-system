package internal

import (
	"context"
	"fmt"
	"image"
	"log"
	"sync"
	"sync/atomic"
	"time"

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
	queue           chan gocv.Mat // Channel for frames
	Signal          signal
	frameCount      int
	frameProcessed  int
	frameSkipped    int
	emptyFrames     int
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
		queue:           make(chan gocv.Mat, config.QueueSize),
		Signal:          signal{Done: make(chan struct{})},
		capture:         capture,
		frameCount:      0,
		emptyFrames:     0,
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
				if vi.emptyFrames > 10 {
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

			select {
			case vi.queue <- resized:
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

			if vi.frameCount >= vi.config.MaxTotalFrames {
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
		case frame, ok := <-vi.queue:
			if !ok {
				log.Printf("frame queue closed")
				return
			}

			if vi.frameProcessed%100 == 0 {
				timestamp := time.Now().UnixNano()
				filename := fmt.Sprintf("/tmp/imgs/output_%d.jpg", timestamp)
				if ok := gocv.IMWrite(filename, frame); !ok {
					log.Printf("Failed to write frame to file")
				}
				log.Printf("Frame [%d] proccessed.\n", vi.frameProcessed+1)
			}
			frame.Close()
			vi.frameProcessed++
			// buf, err := gocv.IMEncode(".jpg", frame)
			// if err != nil {
			// 	return fmt.Errorf("failed to encode frame: %v", err)
			// }
			// defer buf.Close()

			// if err := SendFrame(buf.GetBytes(), vi.grpcDtClientRef); err != nil {
			// 	return fmt.Errorf("failed to send frame: %v", err)
			// }

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
	for frame := range vi.queue {
		frame.Close() // cleanup frames left in the queue
	}

	log.Println("VideoInput closed.")
}

// SendFrame sends a frame to the remote service
func SendFrame(frame []byte, clientRef *atomic.Value) error {
	clientIface := clientRef.Load()
	if clientIface == nil {
		return fmt.Errorf("client is not initialized")
	}
	client := clientIface.(pb.DetectionTrackingPipelineClient)

	f := &pb.FrameData{
		FrameData:     frame,
		SentTimestamp: fmt.Sprintf("%d", int(time.Now().UnixMilli())),
	}

	pong, err := client.SendFrameServer(context.Background(), f)
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
