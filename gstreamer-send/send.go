package send

import (
	"fmt"
	"strings"

	"github.com/lucsky/cuid"
	gst "github.com/pion/ion-sdk-go/pkg/gstreamer-src"

	util "github.com/manishiitg/actions/util"
	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/webrtc/v3"
)

func Init(addr, session, audioSrc, videoSrc string, cancel chan struct{}) *sdk.Engine {
	e := util.GetEngine()
	go run(e, addr, session, audioSrc, videoSrc, cancel)
	return e
}

func run(e *sdk.Engine, addr, session, audioSrc, videoSrc string, cancel chan struct{}) {
	log.Infof("AddClient session=%v addr=%v", session, addr)

	notify := make(chan string, 1)
	go util.GetHost(addr, session, notify, cancel, "sub", -1)
	sfu_host := <-notify

	if strings.Index(sfu_host, "=") != -1 {
		session = strings.Split(sfu_host, "=")[1]
		sfu_host = strings.Split(sfu_host, "=")[0]
	}

	cid1 := fmt.Sprintf("%s_mirror_source_%s", session, cuid.New())
	c, err := sdk.NewClient(e, sfu_host, cid1)
	if err != nil {
		log.Errorf("err=%v", err)
		return
	}

	var peerConnection *webrtc.PeerConnection = c.GetPubTransport().GetPeerConnection()

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Infof("Connection state changed: %s", state)
	})

	if err != nil {
		log.Errorf("client err=%v", err)
		panic(err)
	}

	err = e.AddClient(c)
	if err != nil {
		return
	}

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	if err != nil {
		panic(err)
	}

	_, err = peerConnection.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		panic(err)
	}

	// client join a session
	err = c.Join(session, nil)

	if err != nil {
		log.Errorf("join err=%v", err)
		panic(err)
	}

	// Start pushing buffers on these tracks
	gst.CreatePipeline("opus", []*webrtc.TrackLocalStaticSample{audioTrack}, audioSrc).Start()
	gst.CreatePipeline("vp8", []*webrtc.TrackLocalStaticSample{videoTrack}, videoSrc).Start()

	select {}

}
