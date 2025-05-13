package utils

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	api "github.com/etesami/detection-tracking-system/api"
	pb "github.com/etesami/detection-tracking-system/pkg/protoc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DownloadRandomVideoFile downloads a random video file from a given URL
// The URL should point to a text file containing a list of video file names in the same directory
func DownloadRandomVideoFile(fileNameURL string) (string, error) {

	resp, err := http.Get(fileNameURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(buf), "\n")
	selected := strings.TrimSpace(lines[rand.Intn(len(lines))])
	if selected == "" {
		return "", fmt.Errorf("no video file selected")
	}
	log.Printf("Selected video file: %s\n", selected)

	parsedURL, err := url.Parse(fileNameURL)
	if err != nil {
		return "", err
	}

	// Remove the last segment from the path
	parsedURL.Path = path.Dir(parsedURL.Path)

	// Convert back to string
	baseURL := parsedURL.String()

	fileURL := baseURL + "/" + selected
	log.Printf("File URL: %s\n", fileURL)

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "downloaded_files")
	if err != nil {
		return "", err
	}

	// Full path for the downloaded file
	filePath := tempDir + "/" + selected
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err = http.Get(fileURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	return filePath, err
}

// calculateRtt calculates the round-trip time (RTT) based on the current time and the ack time
// All timestamps are in RFC3339Nano format
func CalculateRtt(msgSentTime, msgRecTime, ackSentTime, ackRecTime string) (float64, error) {
	msgSentTime1, err1 := time.Parse(time.RFC3339Nano, msgSentTime)
	msgRecTime1, err2 := time.Parse(time.RFC3339Nano, msgRecTime)
	ackSentTime1, err3 := time.Parse(time.RFC3339Nano, ackSentTime)
	ackRecTime1, err4 := time.Parse(time.RFC3339Nano, ackRecTime)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return -1, fmt.Errorf("error parsing timestamps: (%v, %v, %v)", err1, err2, err3)
	}
	t1 := msgRecTime1.Sub(msgSentTime1)
	t2 := ackRecTime1.Sub(ackSentTime1)
	rtt := t1 + t2
	return float64(rtt.Microseconds()) / 1000.0, nil
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

func GetPodIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", nil
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

type GrpcClient struct {
	mu     sync.Mutex
	client pb.DetectionTrackingPipelineClient
}

func (g *GrpcClient) Store(client pb.DetectionTrackingPipelineClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.client = client
}

func (g *GrpcClient) Load() pb.DetectionTrackingPipelineClient {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.client
}

func MonitorConnection1(targetSvc api.Service, clientRef *GrpcClient) {
	var conn *grpc.ClientConn

	for {
		if err := targetSvc.ServiceReachable(); err != nil {
			if conn != nil {
				conn.Close()
			}
			clientRef.Store(nil)
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
		}
		time.Sleep(5 * time.Second)
	}
}

func MonitorConnection(targetSvc api.Service, clientRef *atomic.Value) {
	var conn *grpc.ClientConn

	for {
		if err := targetSvc.ServiceReachable(); err != nil {
			if conn != nil {
				conn.Close()
			}
			clientRef.Store(nil)
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
			// log.Println("gRPC client connected and stored")
		}
		time.Sleep(5 * time.Second)
	}
}
