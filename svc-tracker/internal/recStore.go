package internal

import (
	"image"
	"sync"

	"gocv.io/x/gocv"
	"gocv.io/x/gocv/contrib"
)

type TrackerClient struct {
	mu              sync.Mutex
	sourceId        string
	trackerInstance sync.Map
	lastId          uint64
}

type TrackerInstance struct {
	mu      sync.Mutex
	tracker *gocv.Tracker
	store   image.Rectangle
}

func (tc *TrackerClient) DeleteInstanceAt(index uint64) {
	if i, f := tc.trackerInstance.LoadAndDelete(index); f {
		if ti, ok := i.(*TrackerInstance); ok {
			ti.deleteInstance()
		}
	}
}

func (tc *TrackerClient) AddInstance(instance *TrackerInstance) {
	if instance == nil {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.lastId++
	tc.trackerInstance.Store(tc.lastId, instance)
}

func NewTrackerInstance(rec image.Rectangle) *TrackerInstance {
	tracker := contrib.NewTrackerKCF()
	return &TrackerInstance{
		tracker: &tracker,
		store:   rec,
	}
}

func (ti *TrackerInstance) InitTracker(frame gocv.Mat) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	if ti.tracker != nil {
		(*ti.tracker).Init(frame, ti.store)
	}
}

func (ti *TrackerInstance) UpdateTracker(frame gocv.Mat) bool {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	if ti.tracker != nil {
		rec, ok := (*ti.tracker).Update(frame)
		if ok {
			ti.store = rec
			return true
		}
		return false
	}
	return false
}

func (ti *TrackerInstance) deleteInstance() {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	if ti.tracker != nil {
		(*ti.tracker).Close() // release native resources
		ti.tracker = nil      // clear the pointer
	}
}
