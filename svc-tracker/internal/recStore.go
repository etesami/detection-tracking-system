package internal

import (
	"fmt"
	"image"
	"sync"

	"gocv.io/x/gocv"
	"gocv.io/x/gocv/contrib"
)

type TrackerClient struct {
	mu              sync.Mutex
	sourceId        string
	trackerInstance []*TrackerInstance
}

type TrackerInstance struct {
	mu      sync.Mutex
	tracker *gocv.Tracker
	store   image.Rectangle
}

func (tc *TrackerClient) DeleteInstanceAt(index int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if index < 0 || index >= len(tc.trackerInstance) {
		return // out of bounds, ignore or handle error
	}

	instance := tc.trackerInstance[index]
	if instance != nil {
		instance.DeleteInstance()
	}

	tc.trackerInstance = append(tc.trackerInstance[:index], tc.trackerInstance[index+1:]...)
}

func (tc *TrackerClient) AddInstance(instance *TrackerInstance) {
	if instance == nil {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.trackerInstance = append(tc.trackerInstance, instance)
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

func (ti *TrackerInstance) UpdateTracker(frame gocv.Mat) (bool, error) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	if ti.tracker != nil {
		rec, ok := (*ti.tracker).Update(frame)
		if ok {
			ti.store = rec
			return true, nil
		}
		return false, fmt.Errorf("tracker update failed")
	}
	return false, fmt.Errorf("tracker is nil")
}

func (ti *TrackerInstance) DeleteInstance() {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if ti.tracker != nil {
		(*ti.tracker).Close() // release native resources
		ti.tracker = nil      // clear the pointer
	}
}
