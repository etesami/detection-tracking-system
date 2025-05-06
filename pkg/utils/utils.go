package utils

import (
	"fmt"
	"log"
	"math"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// calculateRtt calculates the round-trip time (RTT) based on the current time and the ack time
// All timestamps are in time.Time format
func CalculateRtt(msgSentTime, msgRecTime, ackSentTime, ackRecTime time.Time) (float64, error) {
	// msgSentTime1, err1 := time.Parse(time.RFC3339, msgSentTime)
	// msgRecTime1, err2 := time.Parse(time.RFC3339, msgRecTime)
	// ackSentTime1, err3 := time.Parse(time.RFC3339, ackSentTime)
	// if err1 != nil || err2 != nil || err3 != nil {
	// 	return -1, fmt.Errorf("error parsing timestamps: (%v, %v, %v)", err1, err2, err3)
	// }
	t1 := msgRecTime.Sub(msgSentTime)
	t2 := ackRecTime.Sub(ackSentTime)
	rtt := float64(t1+t2) / 1000.0
	return rtt, nil
}

func StrUnixToTime(unixStr string) (time.Time, error) {
	unixInt, err := strconv.ParseInt(unixStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse unix time: %v", err)
	}
	return UnixMilliToTime(unixInt), nil
}

// UnixMilliToTime converts a Unix timestamp in milliseconds to a time.Time object
func UnixMilliToTime(unixMilli int64) time.Time {
	return time.Unix(unixMilli/1000, (unixMilli%1000)*int64(time.Millisecond))
}

// ParseBuckets parses a comma-separated string of bucket values into a slice of float64
func ParseBuckets(env string) []float64 {
	if env == "" {
		return nil
	}
	parts := strings.Split(env, ",")
	var buckets []float64
	for _, p := range parts {
		if f, err := strconv.ParseFloat(strings.TrimSpace(p), 64); err == nil {
			buckets = append(buckets, f)
		} else {
			// print error
			fmt.Printf("Error parsing bucket value '%s': %v\n", p, err)
			return nil
		}
	}
	return buckets
}

func GetOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("failed to get outbound IP: %v", err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

type BoundingBox struct {
	X1, Y1, X2, Y2 float64
}

// GetIoU calculates the Intersection over Union (IoU) of two bounding boxes
// Returns 0.0 if the boxes are invalid or if they do not overlap
// Keys: {'x1', 'x2', 'y1', 'y2'}
//
//	The (x1, y1) position is at the top left corner,
//	The (x2, y2) position is at the bottom right corner
//
// Keys: {'x1', 'x2', 'y1', 'y2'}
//
//	The (x, y) position is at the top left corner,
//	The (x2, y2) position is at the bottom right corner
func GetIoU(bb1, bb2 BoundingBox) float64 {
	if bb1.X1 >= bb1.X2 || bb1.Y1 >= bb1.Y2 || bb2.X1 >= bb2.X2 || bb2.Y1 >= bb2.Y2 {
		return 0.0
	}

	xLeft := math.Max(bb1.X1, bb2.X1)
	yTop := math.Max(bb1.Y1, bb2.Y1)
	xRight := math.Min(bb1.X2, bb2.X2)
	yBottom := math.Min(bb1.Y2, bb2.Y2)

	if xRight < xLeft || yBottom < yTop {
		return 0.0
	}

	intersectionArea := (xRight - xLeft) * (yBottom - yTop)
	bb1Area := (bb1.X2 - bb1.X1) * (bb1.Y2 - bb1.Y1)
	bb2Area := (bb2.X2 - bb2.X1) * (bb2.Y2 - bb2.Y1)

	iou := intersectionArea / (bb1Area + bb2Area - intersectionArea)
	if iou >= 0.0 && iou <= 1.0 {
		return iou
	}
	return 0.0
}

func MonitorConnection(targetSvc api.Service, clientRef *atomic.Value) {
	var conn *grpc.ClientConn

	for {
		if err := targetSvc.ServiceReachable(); err != nil {
			if conn != nil {
				conn.Close()
			}
			log.Printf("Target service [%s:%s] is not reachable: %v", targetSvc.Address, targetSvc.Port, err)
			time.Sleep(5 * time.Second)
			continue
		}

		if conn == nil || conn.GetState().String() != "READY" {
			if conn != nil {
				conn.Close()
			}
			newConn, err := grpc.NewClient(
				targetSvc.Address+":"+targetSvc.Port,
				grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Println("Failed to connect:", err)
				time.Sleep(5 * time.Second)
				continue
			}

			conn = newConn
			client := pb.NewDetectionTrackingPipelineClient(conn)
			clientRef.Store(client)
			log.Println("gRPC client connected and stored")
		}
		time.Sleep(5 * time.Second)
	}
}
