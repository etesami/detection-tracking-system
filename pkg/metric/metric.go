package metric

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (

	// should be initialized in main
	sentDataBytesHistogram *prometheus.HistogramVec
	procTimeHistogram      prometheus.Histogram
	rttTimeHistogram       *prometheus.HistogramVec

	procTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "processing_time_ms",
			Help: "Gauge of processing times.",
		},
	)
	rTTTimes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "rtt_times_ms",
			Help: "Gauge of round-trip times for different services.",
		},
		[]string{"service"})
)

func (m *Metric) RegisterMetrics(sentDataBuckets, procTimeBuckets, rttTimeBuckets []float64) {

	if sentDataBuckets == nil {
		sentDataBuckets = prometheus.DefBuckets
	}
	if procTimeBuckets == nil {
		procTimeBuckets = prometheus.DefBuckets
	}
	if rttTimeBuckets == nil {
		rttTimeBuckets = prometheus.DefBuckets
	}

	sentDataBytesHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sent_data_bytes_histogram",
			Help:    "Histogram of sent data bytes.",
			Buckets: sentDataBuckets,
		},
		[]string{"service"},
	)
	procTimeHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "processing_time_ms_histogram",
			Help:    "Histogram of processing times.",
			Buckets: procTimeBuckets,
		},
	)
	rttTimeHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rtt_times_ms_histogram",
			Help:    "Histogram of round-trip times.",
			Buckets: rttTimeBuckets,
		},
		[]string{"service"},
	)

	// Register the metrics with Prometheus
	prometheus.MustRegister(sentDataBytesHistogram)
	prometheus.MustRegister(procTimeHistogram)
	prometheus.MustRegister(rttTimeHistogram)
	prometheus.MustRegister(procTime)
	prometheus.MustRegister(rTTTimes)
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
	procTimeHistogram.Observe(time)
	procTime.Set(time)
}

func (m *Metric) AddRttTime(s string, time float64) {
	m.lock()
	defer m.unlock()
	rttTimeHistogram.WithLabelValues(s).Observe(time)
	rTTTimes.WithLabelValues(s).Set(time)
}

func (m *Metric) lock() {
	m.mu.Lock()
}

func (m *Metric) unlock() {
	m.mu.Unlock()
}
