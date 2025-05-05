package internal

import (
	"sync"

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
	queue       chan gocv.Mat
	done        chan struct{}
	frameCount  int
	emptyFrames int
	capture     *gocv.VideoCapture
	wg          sync.WaitGroup
}

// // NewVideoInput creates and initializes a new VideoInput instance
// func NewVideoInput(config *Config) *VideoInput {
// 	vi := &VideoInput{
// 		config:      config,
// 		queue:       make(chan gocv.Mat, config.QueueSize),
// 		done:        make(chan struct{}),
// 		frameCount:  -1,
// 		emptyFrames: 0,
// 	}

// 	vi.openSource()

// 	vi.wg.Add(1)
// 	go vi.processFrames()

// 	return vi
// }

// // openSource attempts to open the video source
// func (vi *VideoInput) openSource() {
// 	var err error
// 	vi.capture, err = gocv.OpenVideoCapture(vi.config.VideoSource)
// 	if err != nil {
// 		log.Printf("Error opening video source: %v\n", err)
// 		return
// 	}
// 	log.Printf("Opened video source: %s\n", vi.config.VideoSource)
// }

// // processFrames reads frames from the video source and processes them
// func (vi *VideoInput) processFrames() {
// 	defer vi.wg.Done()

// 	img := gocv.NewMat()
// 	defer img.Close()

// 	for {
// 		select {
// 		case <-vi.done:
// 			log.Println("Stopping video input processing")
// 			return
// 		default:
// 			// Continue processing frames
// 			if ok := vi.capture.Read(&img); !ok || img.Empty() {
// 				log.Println("Error reading frame")
// 				vi.emptyFrames++
// 				if vi.emptyFrames > 10 {
// 					log.Println("Too many empty frames, stopping video input")
// 					return
// 				}
// 				time.Sleep(500 * time.Millisecond) // Wait before retrying
// 				continue
// 			}

// 			// Resize the image
// 			resized := gocv.NewMat()
// 			gocv.Resize(img, &resized, image.Pt(vi.config.ImageWidth, vi.config.ImageHeight), 0, 0, gocv.InterpolationDefault)

// 			select {
// 			case vi.queue <- resized:
// 				vi.frameCount++
// 			case <-vi.done:
// 				log.Println("Stopping video input processing")
// 				resized.Close()
// 				return
// 			}
// 		}
// 	}
// }

// func (vi *VideoInput) ReadFrame() (gocv.Mat, bool) {
// 	select {
// 	case frame := <-vi.queue:
// 		log.Printf("Read frame %d from queue\n", vi.frameCount)
// 		return frame, true
// 	case <-vi.done:
// 		log.Println("Stopping video input processing")
// 		return gocv.Mat{}, false
// 	default:
// 		return gocv.Mat{}, false
// 	}
// }

// // Close cleans up resources
// func (vi *VideoInput) Close() {
// 	close(vi.done)     // signal goroutine to stop
// 	vi.wg.Wait()       // wait for goroutine to finish
// 	vi.capture.Close() // close video source
// 	close(vi.queue)    // close channel
// 	log.Println("Video input closed.")
// }
