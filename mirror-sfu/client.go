package mirrorsfu

import (
	"fmt"
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

func InitWithAddress(session, session2, addr string, cancel chan struct{}) {

	//doens't work properly not sure why
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

	done := make(chan struct{})

	// c1.OnDataChannel = func(dc *webrtc.DataChannel) {
	// 	go dctodc(dc, c2)
	// }
	// c2.OnDataChannel = func(dc *webrtc.DataChannel) {
	// 	go dctodc(dc, c1)
	// }
	var onceTrackAudio sync.Once
	var onceTrackVideo sync.Once

	audioBuilder := samplebuilder.New(100, &codecs.OpusPacket{}, 48000)
	videoBuilder := samplebuilder.New(100, &codecs.VP8Packet{}, 90000)

	c1.OnTrack = func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			onceTrackAudio.Do(func() {
				tracktotrack(track, receiver, c2, done, cid1, audioBuilder)
				pliLoop(c1, track, 1000)
			})
		}
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			onceTrackVideo.Do(func() {
				tracktotrack(track, receiver, c2, done, cid1, videoBuilder)
				pliLoop(c1, track, 1000)
			})
		}

	}
	// c2.OnTrack = func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	// 	go tracktotrack(track, receiver, c1, done, cid2)
	// }

	err = c1.Join(session, nil)
	defer c1.Close()
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}
	log.Infof("c1 joined session %v", session)

	err = c2.Join(session2, nil)
	defer c2.Close()
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}
	log.Infof("c2 joined  session %v", session2)
	log.Infof("mirroring now")

	ticker := time.NewTicker(10 * time.Second)
	go e.Stats(3, cancel)
	defer ticker.Stop()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case sig := <-sigs:
			log.Infof("Got signal, beginning shutdown", "signal", sig)
			close(cancel)
			close(done)
			time.AfterFunc(10*time.Second, func() {
				os.Exit(1)
			})
		case <-done:
			log.Infof("mirror finished session %v addr1 %v session2 %v addr2 %v", session, addr, session2)
			return
		case <-ticker.C:
			lock.Lock()
			no1 := len(tracks[cid1])
			no2 := len(tracks[cid2])
			log.Infof("session tracker c1:%v no1:%v c2:%v no2:%v", cid1, no1, cid2, no2)
			lock.Unlock()
			recvBW := recvByte / 10 / 1000
			log.Infof("recvBW %v", recvBW)
			if no1 == 0 { // || no2 == 0
				log.Infof("no tracks found closing")
				close(done)
			} else {
				// fmt.Println("tracks found! ", no1)
			}

		}
	}
}

func Init(session, session2 string, cancel chan struct{}) {
	InitWithAddress(session, session2, "", cancel)
}

var lock sync.Mutex

type TrackMap struct {
	id    string
	track *webrtc.TrackLocalStaticSample
}

var tracks = make(map[string][]TrackMap)

// func dctodc(dc *webrtc.DataChannel, c2 *sdk.Client) {
// 	log.Warnf("New DataChannel %s %d\n", dc.Label())
// 	dcID := fmt.Sprintf("dc %v", dc.Label())
// 	log.Warnf("DCID %v", dcID)
// 	dc2, err := c2.CreateDataChannel(dc.Label())
// 	if err != nil {
// 		return
// 	}
// 	dc.OnClose(func() {
// 		dc2.Close()
// 		return
// 	})
// 	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
// 		log.Warnf("Message from DataChannel %v %v", dc.Label(), string(msg.Data))
// 		dc2.SendText(string(msg.Data))
// 	})
// 	dc2.OnMessage(func(msg webrtc.DataChannelMessage) {
// 		// bi-directional data channels
// 		log.Warnf("back msg %v", string(msg.Data))
// 		dc.SendText(string(msg.Data))
// 	})
// }

var recvByte int

func tracktotrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, c2 *sdk.Client, done chan struct{}, cid string, sampleBuffer *samplebuilder.SampleBuilder) {
	log.Infof("GOT TRACK id%v mime%v kind %v stream %v", track.ID(), track.Codec().MimeType, track.Kind(), track.StreamID())

	newTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: track.Codec().MimeType}, "mirror-"+track.ID(), "mirror-"+track.StreamID())
	if err != nil {
		panic(err)
	}
	lock.Lock()
	tracks[cid] = append(tracks[cid], TrackMap{
		id:    track.ID(),
		track: newTrack,
	})
	lock.Unlock()

	t, err := c2.Publish(newTrack)
	if err != nil {
		log.Errorf("publish err=%v", err)
		return
	}
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := t.Sender().Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
	defer c2.UnPublish(t)
	defer func() {
		log.Infof("unpublish track here")
		lock.Lock()
		alltracks := tracks[cid]
		newtracks := []TrackMap{}
		for _, tr := range alltracks {
			if newTrack.ID() != tr.id {
				newtracks = append(newtracks, tr)
			}
		}
		tracks[cid] = newtracks
		lock.Unlock()
	}()
	// rtpBuf := make([]byte, 1400)

	for {
		select {
		case <-done:
			log.Infof("stopping tracks publishing")
			return
		default:

			rtpPacket, _, err := track.ReadRTP()
			if err != nil {
				log.Infof("track read error")
				return
			}

			sampleBuffer.Push(rtpPacket)
			for {
				sample := sampleBuffer.Pop()
				if sample == nil {
					break
				}

				if track.Kind().String() == "video" {

					// Read VP8 header.
					videoKeyframe := (sample.Data[0]&0x1 == 0)
					if videoKeyframe {
						// Keyframe has frame information.
						raw := uint(sample.Data[6]) | uint(sample.Data[7])<<8 | uint(sample.Data[8])<<16 | uint(sample.Data[9])<<24
						width := int(raw & 0x3FFF)
						height := int((raw >> 16) & 0x3FFF)

						log.Infof("width %v height %v", width, height)

					}
				}

				if writeErr := newTrack.WriteSample(*sample); writeErr != nil {
					panic(writeErr)
				}
			}

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

			// i, _, readErr := track.Read(rtpBuf)
			// if readErr != nil {
			// 	log.Infof("track read error")
			// 	return
			// }
			// if _, err = newTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			// 	panic(err)
			// }

		}
	}
	log.Warnf("unpublish track")
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
		}
	}
}
