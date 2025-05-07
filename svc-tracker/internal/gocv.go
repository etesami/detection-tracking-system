package internal

import "image"

func getIoU(bb1, bb2 image.Rectangle) float64 {
	intersect := bb1.Intersect(bb2)
	if intersect.Empty() {
		return 0.0
	}
	interArea := float64(intersect.Dx() * intersect.Dy())
	unionArea := float64(bb1.Dx()*bb1.Dy() + bb2.Dx()*bb2.Dy() - int(interArea))
	return interArea / unionArea
}

// import (
// 	"fmt"

// 	"gocv.io/x/gocv"
// 	"gocv.io/x/gocv/contrib"
// )

// func NewTrackerInstance() {
// 	// create a tracker instance
// 	// (one of MIL, KCF, TLD, MedianFlow, Boosting, MOSSE or CSRT)
// 	tracker := contrib.NewTrackerKCF()
// 	defer tracker.Close()

// 	// prepare image matrix
// 	img := gocv.NewMat()
// 	defer img.Close()

// 	// read an initial image
// 	if ok := webcam.Read(&img); !ok {
// 		fmt.Printf("cannot read device %v\n", deviceID)
// 		return
// 	}

// 	// let the user mark a ROI to track
// 	rect := gocv.SelectROI("Tracking", img)
// 	if rect.Max.X == 0 {
// 		fmt.Printf("user cancelled roi selection\n")
// 		return
// 	}

// 	// initialize the tracker with the image & the selected roi
// 	init := tracker.Init(img, rect)
// 	if !init {
// 		fmt.Printf("Could not initialize the Tracker")
// 		return
// 	}
// }
