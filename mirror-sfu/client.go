package mirrorsfu

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lucsky/cuid"
	util "github.com/manishiitg/actions/util"
	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media/samplebuilder"
)

var recvByte, sendByte int

// var lock sync.Mutex

// type TrackMap struct {
// 	id    string
// 	track *webrtc.TrackLocalStaticRTP
// }

var tracks = make(map[string]string)

func InitWithAddress(session, session2, addr string, cancel chan struct{}) {
	if util.IsActionRunning() {
		log.Errorf("action already running")
	}
	util.StartAction("mirrorsfu", session)
	e := util.GetEngine()

	notify := make(chan string, 1)
	go util.GetHost(addr, session, notify, cancel, "sub", -1)
	sfu_host := <-notify

	if strings.Index(sfu_host, "=") != -1 {
		session = strings.Split(sfu_host, "=")[1]
		sfu_host = strings.Split(sfu_host, "=")[0]
	}

	cid1 := fmt.Sprintf("%s_mirror_source_%s", session, cuid.New())
	c1, err := sdk.NewClient(e, sfu_host, cid1)
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}

	notify2 := make(chan string, 1)
	go util.GetHost(addr, session2, notify2, cancel, "sub", -1)
	sfu_host2 := <-notify2

	if strings.Index(sfu_host2, "=") != -1 {
		session2 = strings.Split(sfu_host2, "=")[1]
		sfu_host2 = strings.Split(sfu_host2, "=")[0]
	}

	cid2 := fmt.Sprintf("%s_mirror_dest_%s", session, cuid.New())
	c2, err := sdk.NewClient(e, sfu_host2, cid2)
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}

	c1.OnDataChannel = func(dc *webrtc.DataChannel) {
		log.Infof("c1 on data channel")
		go dctodc(dc, c2, c1)
	}
	c2.OnDataChannel = func(dc *webrtc.DataChannel) {
		log.Infof("c2 on data channel %v", dc.Label())
		go dctodc(dc, c1, c2)
	}
	var onceTrackAudio sync.Once
	var onceTrackVideo sync.Once

	audioBuilder := samplebuilder.New(10, &codecs.OpusPacket{}, 48000)
	videoBuilder := samplebuilder.New(10, &codecs.VP8Packet{}, 90000)

	wg := new(sync.WaitGroup)
	wg.Add(1) //only audio can mark this as done
	c1.OnTrack = func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Infof("track %v", track.Kind())
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			onceTrackAudio.Do(func() {
				go tracktotrack(track, receiver, c2, cancel, cid1, audioBuilder)
				go pliLoop(c1, track, 1000)
				log.Infof("audio done")
				wg.Done()
			})
		}
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			onceTrackVideo.Do(func() {
				wg.Wait()
				go tracktotrack(track, receiver, c2, cancel, cid1, videoBuilder)
				go pliLoop(c1, track, 1000)
				time.AfterFunc(time.Second*1, func() {
					util.SendData(c2, "mirror", 0, cid2, true)
				})
				log.Infof("video done")
			})
		}

	}

	err = c1.Join(session, nil)
	defer c1.Close()
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}
	log.Infof("c1 joined session %v", session)

	util.UpdateMeta(session2)
	err = c2.Join(session2, nil)
	defer c2.Close()
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}
	log.Infof("c2 joined  session %v", session2)
	log.Infof("mirroring now")

	go func() {
		ticker2 := time.NewTicker(3 * time.Second)
		defer ticker2.Stop()
		for {
			select {
			case <-cancel:
				return
			case <-ticker2.C:

				recvByte = recvByte / 3 / 1000
				info := ""
				info += fmt.Sprintf("RecvBandWidth: %d KB/s\n", recvByte)
				// info += fmt.Sprintf("SendBandWidth: %d KB/s\n", totalSendBW)
				util.UpdateActionProgress(info)
				// log.Infof(info)
				recvByte = 0
			}
		}
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigs:
			log.Infof("Got signal, beginning shutdown", "signal", sig)
			time.AfterFunc(time.Second, func() {
				os.Exit(1)
			})
			close(cancel)
		case <-cancel:
			util.CloseAction()
			log.Infof("mirror finished session %v addr1 %v session2 %v addr2 %v", session, addr, session2)
			return
		case <-ticker.C:
			no1 := len(tracks)
			// log.Infof("session tracker c1:%v no1:%v", cid1, no1)
			if no1 == 0 { // || no2 == 0
				log.Infof("no tracks found closing")
				close(cancel)
			}

		}
	}
}

func Init(session, session2, addr string, cancel chan struct{}) {
	InitWithAddress(session, session2, addr, cancel)
}

func dctodc(dc *webrtc.DataChannel, cother *sdk.Client, cmine *sdk.Client) {
	log.Infof("New DataChannel %v", dc.Label())
	dcID := fmt.Sprintf("dc %v", dc.Label())
	log.Infof("DCID %v", dcID)
	dc2, err := cother.CreateDataChannel(dc.Label() + "-mirror")
	if err != nil {
		log.Errorf("unable to create data channel %v", err)
		return
	}
	dc.OnClose(func() {
		log.Infof("data channel closed!")
		dc2.Close()
		return
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Infof("Message from DataChannel %v %v", dc.Label(), string(msg.Data))
		dc2.Send(msg.Data)
	})

	dc2.OnMessage(func(msg webrtc.DataChannelMessage) {
		// bi-directional data channels
		log.Infof("back msg %v", string(msg.Data))
		dc.Send(msg.Data)
	})
}

func tracktotrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, c2 *sdk.Client, done chan struct{}, cid string, sampleBuffer *samplebuilder.SampleBuilder) {
	log.Infof("GOT TRACK id%v mime%v kind %v stream %v", track.ID(), track.Codec().MimeType, track.Kind(), track.StreamID())

	newTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: track.Codec().MimeType}, "mirror-"+track.ID(), "mirror-"+track.StreamID())
	if err != nil {
		panic(err)
	}
	tracks[track.ID()] = track.Kind().String()
	t, err := c2.Publish(newTrack)
	if err != nil {
		log.Errorf("publish err=%v", err)
		return
	}

	go func() {
		//nack
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := t.Sender().Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
	defer c2.UnPublish(t)
	time.Sleep(time.Second)

	defer func() {
		log.Infof("unpublish track here %v", tracks)
		delete(tracks, track.ID())
	}()
	rtpBuf := make([]byte, 1400)

	for {
		select {
		case <-done:
			log.Infof("stopping tracks publishing")
			return
		default:
			// log.Infof("x")
			// rtpPacket, _, err := track.ReadRTP()
			// if err != nil {
			// 	log.Infof("track read error")
			// 	return
			// }
			// recvByte += len(rtpPacket.Payload)
			// log.Infof("got byptes %v kind %v", recvByte, track.Kind().String())

			// sampleBuffer.Push(rtpPacket)
			// for {
			// 	sample := sampleBuffer.Pop()
			// 	if sample == nil {
			// 		log.Infof("nil sample")
			// 		break
			// 	}

			// 	if track.Kind().String() == "video" {

			// 		// Read VP8 header.
			// 		videoKeyframe := (sample.Data[0]&0x1 == 0)
			// 		if videoKeyframe {
			// 			log.Infof("video key frame")
			// 			// Keyframe has frame information.
			// 			raw := uint(sample.Data[6]) | uint(sample.Data[7])<<8 | uint(sample.Data[8])<<16 | uint(sample.Data[9])<<24
			// 			width := int(raw & 0x3FFF)
			// 			height := int((raw >> 16) & 0x3FFF)

			// 			log.Infof("width %v height %v", width, height)

			// 		}
			// 	}
			// 	log.Infof("writing sample")
			// 	if writeErr := newTrack.WriteSample(*sample); writeErr != nil {
			// 		log.Errorf("write err %v", writeErr)
			// 	}
			// }

			// Read
			// rtpPacket, _, err := track.ReadRTP()
			// if err != nil {
			// 	log.Infof("track read error")
			// 	return
			// }
			// log.Infof("rtpPacket %v", rtpPacket)
			// os.Exit(1)
			// recvByte += len(rtpPacket.Payload)
			// if err = newTrack.WriteRTP(rtpPacket); err != nil {
			// 	log.Infof("track write err", err)
			// 	return
			// }

			i, _, readErr := track.Read(rtpBuf)
			if readErr != nil {
				log.Infof("track read error")
				return
			}
			if i == 0 {
				return
			}
			recvByte += i
			if _, err = newTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				log.Infof("track write err", err)
				return
			}

		}
	}

}

func pliLoop(client *sdk.Client, track *webrtc.TrackRemote, cycle uint) {
	if cycle == 0 {
		cycle = 1000
	}

	ticker := time.NewTicker(time.Duration(cycle) * time.Millisecond)
	for range ticker.C {

		err := client.GetSubTransport().GetPeerConnection().WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{SenderSSRC: uint32(track.SSRC()), MediaSSRC: uint32(track.SSRC())}})
		if err != nil {
			log.Errorf("error writing pli %s", err)
			return
		}
	}
}
