package internal

import (
	"time"

	mt "github.com/etesami/detection-tracking-system/pkg/metric"
)

func addSentDataBytes(l string, m *mt.Metric, sentDataBytes float64) {
	// Calculate the sent data bytes and add it to the metric
	m.AddSentDataBytes(l, sentDataBytes)
}

// E2E Latency (ms) is similar to transit time (ms) but includes the time taken
// to process the frame in the remote service
func addE2ELatency(l string, m *mt.Metric, elapsedMs float64) {
	// Calculate the end-to-end latency and add it to the metric
	m.AddE2ETimes(l, elapsedMs)
}

// Transit time (ms) include the time taken to send the frame
// to the remote service and receive the response, not including
// the processing time in the remote service.
func addTransitTime(l string, m *mt.Metric, elapsedMs float64) {
	// Calculate the transit time and add it to the metric
	m.AddTransitTime(l, elapsedMs)
}

// Processing time metrics includes processing times for readFrames and processFrames
func addProcessingTime(l string, m *mt.Metric, stTime time.Time) {
	// Calculate the processing time and add it to the metric
	elapsed := float64(time.Since(stTime).Microseconds()) / 1000.0
	m.AddProcessingTime(l, elapsed)
}

// Empty frames are frames that are not read from the video source
// More than 10 empty frames in a row will stop the video input
func increaseEmptyFrames(m *mt.Metric) {
	// Increment the empty frames count in the metric
	m.AddFrameCount("empty", 1)
}

// Skipped frames are frames that are not processed due to queue overflow
// and are not sent to the remote service due to issues with the connection or remote service
func increaseSkippedFrames(m *mt.Metric, v *VideoInput) {
	// Increment the skipped frames count in the metric
	m.AddFrameCount("skipped", 1)
	v.frameSkipped++
}

// Processed frames are frames that are successfully read and processed
// including sending to the remote service
func increaseProcessedFrames(m *mt.Metric, v *VideoInput) {
	// Increment the processed frames count in the metric
	m.AddFrameCount("processed", 1)
	v.frameProcessed++
}

// Total frames are all frames read from the video source
// regardless of whether they are empty, skipped, or processed
func increaseTotalFrames(m *mt.Metric, v *VideoInput) {
	// Increment the all frames count in the metric
	m.AddFrameCount("all", 1)
	v.frameCount++
}
