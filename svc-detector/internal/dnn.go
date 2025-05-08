package internal

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"time"

	"gocv.io/x/gocv"
)

var (
	ratio    = 0.003921568627
	mean     = gocv.NewScalar(0, 0, 0, 0)
	swapRGB  = false
	padValue = gocv.NewScalar(144.0, 0, 0, 0)

	scoreThreshold float32 = 0.5
	nmsThreshold   float32 = 0.4
)

func (c *DtConfig) ProcessFrame(frame []byte, frameId int) ([]image.Rectangle, []int) {
	backend := gocv.NetBackendDefault
	target := gocv.NetTargetCPU

	img, err := gocv.IMDecode(frame, gocv.IMReadColor)
	if err != nil {
		log.Printf("Error decoding image: %v", err)
		return nil, nil
	}
	defer img.Close()

	// open DNN object tracking model
	info, err := os.Stat(c.Model)
	if err != nil || info.Size() == 0 {
		log.Fatalf("Model file is missing or empty: %v, %v", c.Model, err)
	}
	net := gocv.ReadNetFromONNX(c.Model)
	if net.Empty() {
		log.Printf("Error reading network model from : %v\n", c.Model)
		return nil, nil
	}
	defer net.Close()
	net.SetPreferableBackend(gocv.NetBackendType(backend))
	net.SetPreferableTarget(gocv.NetTargetType(target))

	outputNames := getOutputNames(&net)
	if len(outputNames) == 0 {
		log.Println("Error reading output layer names")
		return nil, nil
	}

	boxes, indicies := c.detect(&net, &img, outputNames)

	if c.SaveImage && frameId%c.SaveImageFrequency == 0 {
		timestamp := time.Now().UnixNano()
		filename := fmt.Sprintf("%s/%d_detector.jpg", c.SaveImagePath, timestamp)
		if ok := gocv.IMWrite(filename, img); !ok {
			log.Printf("Failed to write frame to file")
		}
		log.Printf("Frame [%d]: Detected %d objects, writtent to [%d_detector.jpg]", frameId, len(boxes), timestamp)
	}
	return boxes, indicies
}

func (c *DtConfig) detect(net *gocv.Net, src *gocv.Mat, outputNames []string) ([]image.Rectangle, []int) {
	params := gocv.NewImageToBlobParams(ratio, image.Pt(c.ImageWidth, c.ImageHeight), mean, swapRGB, gocv.MatTypeCV32F, gocv.DataLayoutNCHW, gocv.PaddingModeLetterbox, padValue)
	blob := gocv.BlobFromImageWithParams(*src, params)
	defer blob.Close()

	// feed the blob into the detector
	net.SetInput(blob, "")

	// run a forward pass thru the network
	probs := net.ForwardLayers(outputNames)
	defer func() {
		for _, prob := range probs {
			prob.Close()
		}
	}()

	boxes, confidences, classIds := performDetection(probs)
	if len(boxes) == 0 {
		log.Println("No classes detected")
		return nil, nil
	}

	iboxes := params.BlobRectsToImageRects(boxes, image.Pt(src.Cols(), src.Rows()))
	indices := gocv.NMSBoxes(iboxes, confidences, scoreThreshold, nmsThreshold)
	drawRects(src, iboxes, classes, classIds, indices)

	return iboxes, indices
}

func getOutputNames(net *gocv.Net) []string {
	var outputLayers []string
	for _, i := range net.GetUnconnectedOutLayers() {
		layer := net.GetLayer(i)
		layerName := layer.GetName()
		if layerName != "_input" {
			outputLayers = append(outputLayers, layerName)
		}
	}

	return outputLayers
}

func performDetection(outs []gocv.Mat) ([]image.Rectangle, []float32, []int) {
	var classIds []int
	var confidences []float32
	var boxes []image.Rectangle

	// needed for yolov8
	gocv.TransposeND(outs[0], []int{0, 2, 1}, &outs[0])

	for _, out := range outs {
		out = out.Reshape(1, out.Size()[1])

		for i := 0; i < out.Rows(); i++ {
			cols := out.Cols()
			scoresCol := out.RowRange(i, i+1)

			scores := scoresCol.ColRange(4, cols)
			_, confidence, _, classIDPoint := gocv.MinMaxLoc(scores)

			if confidence > 0.5 {
				centerX := out.GetFloatAt(i, cols)
				centerY := out.GetFloatAt(i, cols+1)
				width := out.GetFloatAt(i, cols+2)
				height := out.GetFloatAt(i, cols+3)

				left := centerX - width/2
				top := centerY - height/2
				right := centerX + width/2
				bottom := centerY + height/2
				classIds = append(classIds, classIDPoint.X)
				confidences = append(confidences, float32(confidence))

				boxes = append(boxes, image.Rect(int(left), int(top), int(right), int(bottom)))
			}
		}
	}

	return boxes, confidences, classIds
}

func drawRects(img *gocv.Mat, boxes []image.Rectangle, classes []string, classIds []int, indices []int) []string {
	var detectClass []string
	for _, idx := range indices {
		if idx == 0 {
			continue
		}
		gocv.Rectangle(img, image.Rect(boxes[idx].Min.X, boxes[idx].Min.Y, boxes[idx].Max.X, boxes[idx].Max.Y), color.RGBA{0, 255, 0, 0}, 2)
		gocv.PutText(img, classes[classIds[idx]], image.Point{boxes[idx].Min.X, boxes[idx].Min.Y - 10}, gocv.FontHersheyPlain, 0.6, color.RGBA{0, 255, 0, 0}, 1)
		detectClass = append(detectClass, classes[classIds[idx]])
	}

	return detectClass
}
