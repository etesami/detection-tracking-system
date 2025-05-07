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
