package rtmptotrack

import (
	"fmt"
	"net"
	"strings"

	"github.com/lucsky/cuid"
	util "github.com/manishiitg/actions/util"
	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
)

// https://github.com/pion/ion-sdk-go/pull/18

func Init(session string, addr string, cancel <-chan struct{}) *sdk.Engine {
	e := util.GetEngine()

	notify := make(chan string, 1)
	go util.GetHost(addr, session, notify, cancel, "sub", -1)
	sfu_host := <-notify

	if strings.Index(sfu_host, "=") != -1 {
		session = strings.Split(sfu_host, "=")[1]
		sfu_host = strings.Split(sfu_host, "=")[0]
	}

	// create a new client from engine
	cid := fmt.Sprintf("%s_tracktodisk_%s", session, cuid.New())
	client, err := sdk.NewClient(e, sfu_host, cid)
	if err != nil {
		log.Errorf("err=%v", err)
	}
	go run(e, client, session, cancel)
	return e
}

func run(e *sdk.Engine, client *sdk.Client, session string, cancel <-chan struct{}) {

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "rtmptotrack")
	if err != nil {
		panic(err)
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "rtmptotrack")
	if err != nil {
		panic(err)
	}

	log.Infof("joining session=%v", session)
	client.Join(session, nil)
	t, err := client.Publish(audioTrack)
	processRTCP(t)
	t2, err := client.Publish(videoTrack)
	processRTCP(t2)
	defer e.DelClient(client)

	// startRTMPServer(videoTrack, audioTrack)
	// log.Infof("starting rtmp")

	go rtpToTrack(audioTrack, &codecs.OpusPacket{}, 48000, 3006)
	go rtpToTrack(videoTrack, &codecs.VP8Packet{}, 90000, 3004)

	select {
	case <-cancel:
		return
	}
	log.Infof("closing run")
}

// Listen for incoming packets on a port and write them to a Track
func rtpToTrack(track *webrtc.TrackLocalStaticSample, depacketizer rtp.Depacketizer, sampleRate uint32, port int) {
	// Open a UDP Listener for RTP Packets on port 5004
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		log.Errorf("error %v", err)
		return
	}
	defer func() {
		if listener != nil {
			if err = listener.Close(); err != nil {
				log.Errorf("error %v", err)
			}
		}
	}()

	sampleBuffer := samplebuilder.New(10, depacketizer, sampleRate)

	// Read RTP packets forever and send them to the WebRTC Client
	for {
		inboundRTPPacket := make([]byte, 1500) // UDP MTU
		packet := &rtp.Packet{}

		n, _, err := listener.ReadFrom(inboundRTPPacket)
		if err != nil {
			log.Errorf("error during read: %v", err)
			return
		}

		if err = packet.Unmarshal(inboundRTPPacket[:n]); err != nil {
			log.Errorf("error during Unmarshal: %v", err)
		}

		sampleBuffer.Push(packet)
		for {
			sample := sampleBuffer.Pop()
			if sample == nil {
				break
			}
			// log.Infof("sample payload length %v", len(sample.Data))

			if writeErr := track.WriteSample(*sample); writeErr != nil {
				log.Errorf("error during WriteSample: %v", err)
			}
		}
	}
}

// Read incoming RTCP packets
// Before these packets are retuned they are processed by interceptors. For things
// like NACK this needs to be called.
func processRTCP(rtpSender *webrtc.RTPTransceiver) {
	go func() {
		rtcpBuf := make([]byte, 1500)

		for {
			if _, _, rtcpErr := rtpSender.Sender().Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
}
