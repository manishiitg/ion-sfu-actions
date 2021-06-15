package tracktortp

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	util "github.com/manishiitg/actions/util"

	"github.com/lucsky/cuid"
	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

type udpConn struct {
	conn        *net.UDPConn
	port        int
	payloadType uint8
	track       bool
}

func InitApi(serverIp string, session string, rtmp string, cancel chan struct{}) (*sdk.Engine, error) {
	if len(rtmp) == 0 {
		rtmp = "rtmp://bom01.contribute.live-video.net/app/live_666332364_5791UvimKkDZW8edq8DAi4011wc4cR" //TODO
	}
	return Init(session, serverIp, rtmp, cancel)
}

func Init(session string, addr string, rtmp string, cancel chan struct{}) (*sdk.Engine, error) {
	e := util.GetEngine()

	notify := make(chan string, 1)
	go util.GetHost(addr, session, notify, cancel, "sub", -1)
	sfu_host := <-notify

	if strings.Index(sfu_host, "=") != -1 {
		session = strings.Split(sfu_host, "=")[1]
		sfu_host = strings.Split(sfu_host, "=")[0]
	}

	// create a new client from engine

	var err error
	var laddr *net.UDPAddr
	if laddr, err = net.ResolveUDPAddr("udp", "127.0.0.1:"); err != nil {
		log.Errorf("err=%v", err)
		return e, err
	}

	udpConns := map[string]*udpConn{
		"audio": {port: 4000, payloadType: 111, track: false},
		"video": {port: 4002, payloadType: 96, track: false},
	}
	for _, c := range udpConns {
		// Create remote addr
		var raddr *net.UDPAddr
		if raddr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", c.port)); err != nil {
			log.Errorf("err %v", err)
			util.ErrorAction(err)
			return e, err
		}

		// Dial udp
		if c.conn, err = net.DialUDP("udp", laddr, raddr); err != nil {
			log.Errorf("err %v", err)
			util.ErrorAction(err)
			return e, err
		}
	}

	cid := fmt.Sprintf("%s_tracktortp_%s", session, cuid.New())
	client, err := sdk.NewClient(e, sfu_host, cid)
	if err != nil {
		log.Errorf("err=%v sfu host %v", err, sfu_host)
		return e, err
	}

	go run(e, client, rtmp, session, cancel, udpConns)
	return e, nil
}
func run(e *sdk.Engine, client *sdk.Client, rtmp string, session string, cancel chan struct{}, udpConns map[string]*udpConn) {
	// subscribe rtp from sessoin
	// comment this if you don't need save to file

	if util.IsActionRunning() {
		log.Errorf("action already running")
	}
	util.StartAction("tracktortp", session)

	for _, c := range udpConns {
		defer func(conn net.PacketConn) {
			if closeErr := conn.Close(); closeErr != nil {
				log.Errorf("err %v", closeErr)
			}
		}(c.conn)
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	isstarted := false
	//clear if any orphan tracks
	for idx, _ := range rtpTrackMap {
		delete(rtpTrackMap, idx)
	}

	var oncePublish sync.Once
	var onceTrackAudio sync.Once
	var onceTrackVideo sync.Once
	client.OnTrack = func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		oncePublish.Do(func() {
			go publishtortmp(ctx, rtmp)
		})
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			onceTrackAudio.Do(func() {
				go processTrack(track, receiver, cancel, udpConns, client)
			})
		}
		if track.Kind() == webrtc.RTPCodecTypeVideo {
			onceTrackVideo.Do(func() {
				go processTrack(track, receiver, cancel, udpConns, client)
			})
		}
		isstarted = true
	}

	log.Infof("joining session=%v", session)
	err := client.Join(session, nil)
	defer e.DelClient(client)
	if err != nil {
		log.Errorf("err=%v", err)
		util.ErrorAction(err)
	}

	time.AfterFunc(time.Second*60, func() {
		isstarted = true
	})
	timer := time.NewTicker(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			log.Infof("isstarted %v rtpTrackMap %v", isstarted, len(rtpTrackMap))
			if isstarted && len(rtpTrackMap) == 0 {
				log.Infof("tracks closed!")
				close(cancel)
			}
		case <-cancel:
			ctxCancel()
			util.CloseAction()
			log.Infof("closed!")
			return
		}

	}
	log.Infof("closing track to rtmp")
}

var rtpTrackMap = make(map[string]string)

func processTrack(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver, cancel <-chan struct{}, udpConns map[string]*udpConn, client *sdk.Client) {
	log.Infof("GOT TRACK %v", track.Kind())
	c, ok := udpConns[track.Kind().String()]
	if !ok {
		return
	}
	c.track = true
	udpConns[track.Kind().String()] = c
	rtpTrackMap[track.ID()] = track.Kind().String()

	// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
	go func() {
		ticker := time.NewTicker(time.Second * 2)
		for {
			select {
			case <-ticker.C:
				// We need to add direct access to the peerconnection to ion-sdk-go to support PLI here
				// PLI is disabled in this example currently

				if rtcpErr := client.GetSubTransport().GetPeerConnection().WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}}); rtcpErr != nil {
					fmt.Println(rtcpErr)
				}
			case <-cancel:
				return
			}

		}
	}()

	b := make([]byte, 1500)
	rtpPacket := &rtp.Packet{}
	for {
		// Read
		var err error
		n, _, readErr := track.Read(b)
		if readErr != nil {
			log.Errorf("readErr %v", readErr)
			break
		}

		// Unmarshal the packet and update the PayloadType
		if err = rtpPacket.Unmarshal(b[:n]); err != nil {
			log.Errorf("error %v", err)
		}
		rtpPacket.PayloadType = c.payloadType

		// Marshal into original buffer with updated PayloadType
		if n, err = rtpPacket.MarshalTo(b); err != nil {
			log.Errorf("error %v", err)
		}

		// Write
		if _, err = c.conn.Write(b[:n]); err != nil {
			// For this particular example, third party applications usually timeout after a short
			// amount of time during which the user doesn't have enough time to provide the answer
			// to the browser.
			// That's why, for this particular example, the user first needs to provide the answer
			// to the browser then open the third party application. Therefore we must not kill
			// the forward on "connection refused" errors
			opError, ok := err.(*net.OpError)
			if ok && opError.Err.Error() == "write: connection refused" {
				continue
			}
			log.Errorf("error %v", err)
			log.Errorf("opError %v", opError)
			if opError.Err.Error() == "use of closed network connection" {
				break
			}
		}
	}
	defer func() {
		log.Infof("track stopped id %v kind %v", track.ID(), track.Kind().String())
		delete(rtpTrackMap, track.ID())
	}()
}

func publishtortmp(ctx context.Context, streamURL string) error {
	// # ffmpeg -protocol_whitelist file,udp,rtp -i subscribe.sdp -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -ar 44100 -f flv rtmp://bom01.contribute.live-video.net/app/live_666332364_5791UvimKkDZW8edq8DAi4011wc4cR

	log.Infof("publish rtmp %v", streamURL)
	args := "-protocol_whitelist file,udp,rtp -i ./tracktortp/subscribe.sdp -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -ar 44100 -f flv"
	ffmpeg := exec.CommandContext(ctx, "ffmpeg", append(strings.Split(args, " "), streamURL)...)

	ffmpegOut, _ := ffmpeg.StderrPipe()
	if err := ffmpeg.Start(); err != nil {
		log.Infof("err %v", err)
		return err
	}

	go func() {
		scanner := bufio.NewScanner(ffmpegOut)
		for scanner.Scan() {
			txt := scanner.Text()
			fmt.Println(txt)
			util.UpdateActionProgress(txt)
			if ctx.Err() == context.Canceled {
				log.Infof("context cancelled")
				break
			}
		}
	}()

	// err := ffmpeg.Wait()
	// if err != nil {
	// 	log.Errorf("err %v", err)
	// }
	return nil
}
