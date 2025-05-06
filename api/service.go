package api

import (
	"fmt"
	"net"
	"time"
)

type FrameMetadata struct {
	Timestamp string `json:"timestamp,omitempty"`
	SourceId  string `json:"source_id,omitempty"`
	FrameId   int64  `json:"frame_id,omitempty"`
}

type Service struct {
	Address string
	Port    string
}

func (s *Service) ServiceReachable() error {
	if s.Address == "" || s.Port == "" {
		return fmt.Errorf("service address or port is not set")
	}
	address := fmt.Sprintf("%s:%s", s.Address, s.Port)
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}
