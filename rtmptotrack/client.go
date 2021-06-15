package rtmptotrack

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

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

func Init(session string, addr string, rtmpInput string, cancel <-chan struct{}) *sdk.Engine {
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
		log.Errorf("err=%v sfu host %v", err, sfu_host)
	}
	go run(e, client, session, rtmpInput, cancel)
	return e
}

func run(e *sdk.Engine, client *sdk.Client, session string, rtmpInput string, cancel <-chan struct{}) {
	if util.IsActionRunning() {
		log.Errorf("action already running")
	}
	util.StartAction("rtmptotrack", session)
	ctx, ctxCancel := context.WithCancel(context.Background())
	wait := make(chan struct{})
	if rtmpInput == "demo" {
		go streamMp4ToRtmp(ctx, wait)
	} else {
		go rtmpToRtp(rtmpInput, ctx, wait)
	}
	log.Infof("waiting for ffmpeg to start!")
	select {
	case <-wait:
		log.Infof("ffmpeg started")
	case <-time.After(30 * time.Second):
		log.Infof("ffmpeg didn't start for 30sec some problem!")
	}
	time.Sleep(1 * time.Second) // quite strange if i don't add this sleep it doesn't work

	uniq := cuid.New()
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "rtmptotrack"+uniq)
	if err != nil {
		log.Errorf("err", err)
		util.ErrorAction(err)
		ctxCancel()
		return
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "rtmptotrack"+uniq)
	if err != nil {
		log.Errorf("err", err)
		ctxCancel()
		util.ErrorAction(err)
		return
	}

	log.Infof("joining session=%v", session)
	client.Join(session, nil)
	defer e.DelClient(client)
	// t, err := client.Publish(audioTrack)
	// processRTCP(t)
	// t2, err := client.Publish(videoTrack)
	// processRTCP(t2)

	t, _ := client.GetPubTransport().GetPeerConnection().AddTransceiverFromTrack(audioTrack, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	})

	defer client.UnPublish(t)
	t2, _ := client.GetPubTransport().GetPeerConnection().AddTransceiverFromTrack(videoTrack, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionSendonly,
	})
	defer client.UnPublish(t2)
	time.Sleep(5 * time.Millisecond)
	client.OnNegotiationNeeded()

	go rtpToTrack(client, audioTrack, &codecs.OpusPacket{}, 48000, 3006, cancel)
	go rtpToTrack(client, videoTrack, &codecs.VP8Packet{}, 90000, 3004, cancel)

	select {
	case <-cancel:
		ctxCancel()
		util.CloseAction()
		log.Infof("closing run")
		return
	}
}

// Listen for incoming packets on a port and write them to a Track
func rtpToTrack(client *sdk.Client, track *webrtc.TrackLocalStaticSample, depacketizer rtp.Depacketizer, sampleRate uint32, port int, cancel <-chan struct{}) {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		log.Errorf("error %v", err)
		util.ErrorAction(err)
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
		select {
		case <-cancel:
			return
		default:
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

func streamMp4ToRtmp(ctx context.Context, wait chan struct{}) error {
	// ffmpeg -re -stream_loop 400 -i /var/tmp/big_buck_bunny_360p_10mb.mp4 -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -f flv rtmp://localhost:1935/live/rfBd56ti2SMtYvSgD5xAV0YU99zampta7Z7S575KLkIZ9PYk

	path := getDemoFile()
	key, err := getStreamKey()
	if err != nil {
		return err
	}

	if len(key) == 0 {
		log.Errorf("empty stream key!")
		return errors.New("empty stream key!")
	}

	argstr := "-re -stream_loop 1000 -i " + path + " -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -f flv"
	args := append(strings.Split(argstr, " "), "rtmp://localhost:1935/live/"+key)
	log.Infof("args %v", args)
	ffmpeg := exec.CommandContext(ctx, "ffmpeg", args...)

	ffmpegOut, _ := ffmpeg.StderrPipe()
	if err := ffmpeg.Start(); err != nil {
		log.Infof("err %v", err)
		util.ErrorAction(err)
		return err
	}

	go func() {
		scanner := bufio.NewScanner(ffmpegOut)
		for scanner.Scan() {
			// fmt.Println(scanner.Text())
			// log.Infof(scanner.Text())
			if ctx.Err() == context.Canceled {
				log.Infof("context cancelled")
				break
			}
		}
	}()
	time.AfterFunc(1*time.Second, func() {
		go rtmpToRtp("rtmp://localhost:1935/live/movie", ctx, wait)
	})
	return nil
}

func rtmpToRtp(rtmpInput string, ctx context.Context, wait chan struct{}) error {
	// ffmpeg -i rtmp://localhost:1935/live/movie -an -vcodec libvpx -cpu-used 5 -deadline 1 -g 10 -error-resilient 1 -auto-alt-ref 1 -f rtp rtp://127.0.0.1:3004 -vn -c:a libopus -f rtp rtp://127.0.0.1:3006
	args := "-i " + rtmpInput + " -an -vcodec libvpx -cpu-used 5 -deadline 1 -g 10 -error-resilient 1 -auto-alt-ref 1 -f rtp rtp://127.0.0.1:3004 -vn -c:a libopus -f rtp rtp://127.0.0.1:3006"
	log.Infof("ffmpeg %v", args)
	ffmpeg := exec.CommandContext(ctx, "ffmpeg", strings.Split(args, " ")...)

	ffmpegOut, _ := ffmpeg.StderrPipe()
	if err := ffmpeg.Start(); err != nil {
		log.Infof("err %v", err)
		return err
	}
	// ffmpeg.StdoutPipe()

	go func() {
		notified := false
		scanner := bufio.NewScanner(ffmpegOut)
		for scanner.Scan() {
			fps := strings.Contains(scanner.Text(), "fps")
			if fps && !notified {
				close(wait)
				notified = true
			}
			// fmt.Println(scanner.Text())
			// log.Infof(scanner.Text())
			if ctx.Err() == context.Canceled {
				log.Infof("context cancelled")
				break
			}
		}
	}()

	return nil
}

type streamResp struct {
	Status int    `json:"status"`
	Data   string `json:"data"`
}

func getStreamKey() (string, error) {
	resp, err := http.Get("http://localhost:8090/control/get?room=movie")
	if err != nil {
		log.Errorf("http error %v", err)
	}

	body, err2 := ioutil.ReadAll(resp.Body)
	if err2 != nil {
		log.Errorf("%v", err)
		return "", err
	} else {
		log.Infof("body %v", string(body))
		var response streamResp
		err = json.Unmarshal(body, &response)
		if err != nil {
			log.Errorf("error parsing host response", err)
			return "", err
		}
		log.Infof("response from rtmp stream key %v", response)
		return response.Data, nil
	}

}

func getDemoFile() string {
	var filepath string
	filepath = "/var/tmp/big_buck_bunny_360p_10mb.mp4"
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		log.Infof("download file...")
		err := util.DownloadFile(filepath, "https://sample-videos.com/video123/mp4/360/big_buck_bunny_360p_10mb.mp4")
		if err != nil {
			log.Infof("error downloading file %v", err)
		}
		log.Infof("download completed...")
	}
	return filepath
}
