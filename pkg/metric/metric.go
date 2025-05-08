package metric

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (

	// Histograms, should be initialized in main
	e2eTimeHistogram       *prometheus.HistogramVec
	sentDataBytesHistogram *prometheus.HistogramVec
	procTimeHistogram      *prometheus.HistogramVec
	transTimeHistogram     *prometheus.HistogramVec

	frameCountVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "frame_count",
			Help: "Frame count by status.",
		},
		[]string{"status"}, // "processed", "skipped", "empty", "all"
	)
)

func (m *Metric) RegisterMetrics(sentDataBuckets, procTimeBuckets, transitTimeBuckets, e2eTimeBuckets []float64) {

	if sentDataBuckets == nil {
		sentDataBuckets = prometheus.DefBuckets
	}
	if procTimeBuckets == nil {
		procTimeBuckets = prometheus.DefBuckets
	}
	if transitTimeBuckets == nil {
		transitTimeBuckets = prometheus.DefBuckets
	}
	if e2eTimeBuckets == nil {
		e2eTimeBuckets = prometheus.DefBuckets
	}

	e2eTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "e2e_time_ms_histogram",
			Help:    "Histogram of end-to-end latency times.",
			Buckets: e2eTimeBuckets,
		},
		[]string{"service"},
	)

	sentDataBytesHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sent_data_bytes_histogram",
			Help:    "Histogram of sent data bytes.",
			Buckets: sentDataBuckets,
		},
		[]string{"service"},
	)

	procTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "processing_time_ms_histogram",
			Help:    "Histogram of processing latency times.",
			Buckets: procTimeBuckets,
		},
		[]string{"service"},
	)

	transTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "transit_time_ms_histogram",
			Help:    "Histogram of round-trip latency times.",
			Buckets: transitTimeBuckets,
		},
		[]string{"service"},
	)

	// Register the metrics with Prometheus
	prometheus.MustRegister(sentDataBytesHistogram)
	prometheus.MustRegister(procTimeHistogram)
	prometheus.MustRegister(transTimeHistogram)
	prometheus.MustRegister(e2eTimeHistogram)
	prometheus.MustRegister(frameCountVec)
}

type Metric struct {
	mu sync.Mutex
}

func (m *Metric) AddSentDataBytes(s string, bytes float64) {
	m.lock()
	defer m.unlock()
	sentDataBytesHistogram.WithLabelValues(s).Observe(bytes)
}

func (m *Metric) AddProcessingTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	procTimeHistogram.WithLabelValues(s).Observe(time)
}

func (m *Metric) AddTransitTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	transTimeHistogram.WithLabelValues(s).Observe(time)
}

func (m *Metric) AddE2ETimes(s string, time float64) {
	m.lock()
	defer m.unlock()
	e2eTimeHistogram.WithLabelValues(s).Observe(time)
}

func (m *Metric) AddFrameCount(s string, count float64) {
	m.lock()
	defer m.unlock()
	frameCountVec.WithLabelValues(s).Add(count)
}

func (m *Metric) lock() {
	m.mu.Lock()
}

func (m *Metric) unlock() {
	m.mu.Unlock()
}
