package internal

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/asticode/go-astits"
	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
)

var auCounter int

func findTrack(r *mpegts.Reader) (*mpegts.Track, error) {
	for _, track := range r.Tracks() {
		if _, ok := track.Codec.(*mpegts.CodecH264); ok {
			return track, nil
		}
	}
	return nil, fmt.Errorf("H264 track not found")
}

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func RouteFrames(f *os.File, stream *gortsplib.ServerStream) {
	// setup H264 -> RTP encoder
	rtpEnc, err := stream.Desc.Medias[0].Formats[0].(*format.H264).CreateEncoder()
	if err != nil {
		panic(err)
	}

	randomStart, err := randUint32()
	if err != nil {
		panic(err)
	}

	for {
		// setup MPEG-TS parser
		r := &mpegts.Reader{R: f}
		err = r.Initialize()
		if err != nil {
			panic(err)
		}

		// find the H264 track inside the file
		track, err := findTrack(r)
		if err != nil {
			panic(err)
		}

		timeDecoder := mpegts.TimeDecoder{}
		timeDecoder.Initialize()

		var firstDTS *int64
		var firstTime time.Time
		var lastRTPTime uint32

		// setup a callback that is called when a H264 access unit is read from the file
		r.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
			dts = timeDecoder.Decode(dts)
			pts = timeDecoder.Decode(pts)

			// sleep between access units
			if firstDTS != nil {
				timeDrift := time.Duration(dts-*firstDTS)*time.Second/90000 - time.Since(firstTime)
				if timeDrift > 0 {
					time.Sleep(timeDrift)
				}
			} else {
				firstTime = time.Now()
				firstDTS = &dts
			}

			// increase counter
			auCounter++
			if auCounter%500 == 0 {
				log.Printf("writing access unit with pts=%d dts=%d", pts, dts)
			}

			// wrap the access unit into RTP packets
			packets, err := rtpEnc.Encode(au)
			if err != nil {
				return err
			}

			// set packet timestamp
			// we don't have to perform any conversion
			// since H264 clock rate is the same in both MPEG-TS and RTSP
			lastRTPTime = uint32(int64(randomStart) + pts)
			for _, packet := range packets {
				packet.Timestamp = lastRTPTime
			}

			// write RTP packets to the server
			for _, packet := range packets {
				err := stream.WritePacketRTP(stream.Desc.Medias[0], packet)
				if err != nil {
					return err
				}
			}

			return nil
		})

		// read the file
		for {
			err := r.Read()
			if err != nil {
				// file has ended
				if errors.Is(err, astits.ErrNoMorePackets) {
					log.Printf("file has ended, rewinding")

					// rewind to start position
					_, err = f.Seek(0, io.SeekStart)
					if err != nil {
						panic(err)
					}

					// keep current timestamp
					randomStart = lastRTPTime + 1

					break
				}
				panic(err)
			}
		}
	}
}

type ServerHandler struct {
	Server *gortsplib.Server
	Stream *gortsplib.ServerStream
	Mutex  sync.RWMutex
}

// called when a connection is opened.
func (sh *ServerHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (sh *ServerHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (sh *ServerHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (sh *ServerHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")
}

// called when receiving a DESCRIBE request.
func (sh *ServerHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("DESCRIBE request")

	sh.Mutex.RLock()
	defer sh.Mutex.RUnlock()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.Stream, nil
}

// called when receiving a SETUP request.
func (sh *ServerHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("SETUP request")

	sh.Mutex.RLock()
	defer sh.Mutex.RUnlock()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.Stream, nil
}

// called when receiving a PLAY request.
func (sh *ServerHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("PLAY request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}
