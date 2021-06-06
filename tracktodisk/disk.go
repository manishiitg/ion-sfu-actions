package tracktodisk

import (
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/lucsky/cuid"
	"github.com/manishiitg/actions/loadtest/client/gst"
	util "github.com/manishiitg/actions/util"
	avp "github.com/pion/ion-avp/pkg"
	"github.com/pion/ion-avp/pkg/elements"
	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

func InitApi(serverip string, session string, vtype string, cancel <-chan struct{}) *sdk.Engine {
	return Init(session, serverip, vtype, cancel)
}

func Init(session string, addr string, vtype string, cancel <-chan struct{}) *sdk.Engine {
	e := util.GetEngine()

	notify := make(chan string, 1)
	go util.GetHost(addr, session, notify, cancel, "sub", -1)
	sfu_host := <-notify

	if strings.Index(sfu_host, "=") != -1 {
		session = strings.Split(sfu_host, "=")[1]
		sfu_host = strings.Split(sfu_host, "=")[0]
	}

	go run(e, sfu_host, session, vtype, cancel)
	return e
}

func run(e *sdk.Engine, sfu_host string, session string, vtype string, cancel <-chan struct{}) {

	// create a new client from engine
	cid := fmt.Sprintf("%s_tracktodisk_%s", session, cuid.New())
	client, err := sdk.NewClient(e, sfu_host, cid)
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}

	if vtype == "gstreamer" && false {
		// not working getting error
		//AL lib: (EE) ALCplaybackAlsa_open: Could not open playback device 'default': No such file or directory
		// GStreamer Error: Failed to initialize egl: EGL_NOT_INITIALIZED
		compositeSavePath := "test.mp4"

		encodePipeline := fmt.Sprintf(`
				tee name=aenctee 
				tee name=venctee
				vtee. ! queue ! vtenc_h264 ! video/x-h264,chroma-site=mpeg2 ! venctee.
				atee. ! queue ! faac ! aenctee.
		`)

		encodePipeline += fmt.Sprintf(`
				qtmux name=savemux ! queue ! filesink location=%s async=false sync=false
				venctee. ! queue ! savemux.
				aenctee. ! queue ! savemux. 
			`, compositeSavePath)
		log.Infof("saving encoded stream", "path", compositeSavePath)

		log.Infof("encoding composited stream")

		compositor := gst.NewCompositorPipeline(encodePipeline)
		compositor.Play()
		client.OnTrack = func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			if track.Kind().String() == "video" {
				compositor.AddInputTrack(track, client.GetSubTransport().GetPeerConnection())
			}
		}
		defer compositor.Stop()
	} else {
		var onceTrackAudio sync.Once
		var onceTrackVideo sync.Once

		saver := createWebmSaver(session, cid)
		client.OnTrack = func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			if track.Kind() == webrtc.RTPCodecTypeAudio {
				onceTrackAudio.Do(func() {
					tracktodisk(track, receiver, saver, client)
				})
			}
			if track.Kind() == webrtc.RTPCodecTypeVideo {
				onceTrackVideo.Do(func() {
					tracktodisk(track, receiver, saver, client)
				})
			}

		}

	}

	log.Infof("joining session=%v", session)
	err = client.Join(session, nil)
	defer e.DelClient(client)
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}

	select {
	case <-cancel:
		return
	}
	log.Infof("closed!")
}

func tracktodisk(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, saver avp.Element, client *sdk.Client) {
	log.Infof("got track %v type %v", track.ID(), track.Kind())
	maxPacketsLate := uint16(100)
	maxTimeLate := time.Millisecond * time.Duration(0)
	builder := avp.MustBuilder(avp.NewBuilder(track, maxPacketsLate, avp.WithMaxLateTime(maxTimeLate)))

	if track.Kind() == webrtc.RTPCodecTypeVideo {
		err := client.GetSubTransport().GetPeerConnection().WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{SenderSSRC: uint32(track.SSRC()), MediaSSRC: uint32(track.SSRC())}})
		if err != nil {
			log.Errorf("error writing pli %s", err)
		}
	}
	builder.AttachElement(saver)
	go pliLoop(client, track, 1000)
	builder.OnStop(func() {
		log.Infof("builder stopped")
	})
}

func createWebmSaver(sid, pid string) avp.Element {
	filewriter := elements.NewFileWriter(
		path.Join("./out", fmt.Sprintf("%s-%s.webm", sid, pid)),
		4096,
	)
	webm := elements.NewWebmSaver()
	webm.Attach(filewriter)
	return webm
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
