package loadtest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lucsky/cuid"
	client "github.com/manishiitg/actions/loadtest/client"
	util "github.com/manishiitg/actions/util"
	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/webrtc/v3"
)

func getFileByType(file string) string {
	var filepath string
	log.Infof("got file type %v", file)
	if file == "360p" {
		filepath = "/var/tmp/Big_Buck_Bunny_4K.webm.360p.webm"
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			err := util.DownloadFile(filepath, "https://upload.wikimedia.org/wikipedia/commons/transcoded/c/c0/Big_Buck_Bunny_4K.webm/Big_Buck_Bunny_4K.webm.360p.webm")
			if err != nil {
				log.Infof("error downloading file %v", err)
				filepath = "test"
			}

		}
	}
	if file == "480p" {
		filepath = "/var/tmp/Big_Buck_Bunny_4K.webm.480p.webm"
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			err := util.DownloadFile(filepath, "https://upload.wikimedia.org/wikipedia/commons/transcoded/c/c0/Big_Buck_Bunny_4K.webm/Big_Buck_Bunny_4K.webm.480p.webm")
			if err != nil {
				log.Infof("error downloading file %v", err)
				filepath = "test"
			}
		}
	}

	if file == "720p" {
		filepath = "/var/tmp/Big_Buck_Bunny_4K.webm.720p.webm"
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			err := util.DownloadFile(filepath, "https://upload.wikimedia.org/wikipedia/commons/transcoded/c/c0/Big_Buck_Bunny_4K.webm/Big_Buck_Bunny_4K.webm.720p.webm")
			if err != nil {
				log.Infof("error downloading file %v", err)
				filepath = "test"
			}
		}
	}

	if file == "h264" {
		//TODO not working as of now need to debug
		// load is there but doesn't play on browser. track should play on browser also
		// issue is on browser side
		filepath = "/var/tmp/Big_Buck_Bunny_720_10s_1MB.mp4"
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			err := util.DownloadFile(filepath, "https://test-videos.co.uk/vids/bigbuckbunny/mp4/h264/720/Big_Buck_Bunny_720_10s_1MB.mp4")
			if err != nil {
				log.Infof("error downloading file %v", err)
				filepath = "test"
			}
		}
	}

	if filepath == "" {
		filepath = "test"
	}

	return filepath
}

func InitLoadTestApi(serverIp string, session string, clients int, role string, cycle int, rooms int, file string, capacity int, cancel chan struct{}) *sdk.Engine {
	if clients == 0 {
		clients = 1
	}
	return Init(file, serverIp, session, clients, cycle, 60*5, role, rooms, capacity, cancel)
}

func Init(file, gaddr, session string, total, cycle, duration int, role string, create_room int, capacity int, cancel chan struct{}) *sdk.Engine {
	file = getFileByType(file)
	log.Infof("filepath %v", file)
	video := true
	audio := true
	simulcast := ""

	e := util.GetEngine()
	// if paddr != "" {
	// 	go e.ServePProf(paddr)
	// }
	go run(e, gaddr, session, file, role, total, duration, cycle, video, audio, simulcast, create_room, capacity, cancel)
	return e
}

type profile struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
type message struct {
	Action     string  `json:"action"`
	Id         string  `json:"id"`
	Data       profile `json:"data"`
	Streamid   string  `json:"streamid"`
	IsHost     bool    `json:"ishost"`
	CanPublish bool    `json:"canPublish"`
}

func run(e *sdk.Engine, addr, session, file, role string, total, duration, cycle int, video, audio bool, simulcast string, create_room int, capacity int, cancel chan struct{}) *sdk.Engine {
	log.Infof("run session=%v file=%v role=%v total=%v duration=%v cycle=%v video=%v audio=%v simulcast=%v\n", session, file, role, total, duration, cycle, audio, video, simulcast)
	timer := time.NewTimer(time.Duration(duration) * time.Second)

	if util.IsActionRunning() {
		log.Errorf("action already running")
	}
	util.StartAction("loadtest", session)
	util.UpdateMeta(fmt.Sprintf("%v", total))
	for i := 0; i < total; i++ {
		go func(i int, session string) {
			new_session := session
			if create_room != -1 {
				new_session = new_session + fmt.Sprintf("%v", i%create_room)
			}

			notify := make(chan string, 1)
			go util.GetHost(addr, new_session, notify, cancel, role, capacity)
			sfu_host := <-notify

			if strings.Index(sfu_host, "=") != -1 {
				new_session = strings.Split(sfu_host, "=")[1]
				sfu_host = strings.Split(sfu_host, "=")[0]
			}

			crole := role

			if role == "sub" && i == 0 {
				crole = "pubsub" // even if doing load test of sub, need one publisher at least!
			}

			switch crole {
			case "pubsub":

				cid := fmt.Sprintf("%s_pubsub_%d_%s", new_session, i, cuid.New())
				log.Infof("AddClient session=%v clientid=%v addr=%v", new_session, cid, sfu_host)
				c, err := sdk.NewClient(e, sfu_host, cid)
				if err != nil {
					log.Errorf("%v", err)
					break
				}
				handleDataChannel(c, i, cid)

				if !strings.Contains(file, ".webm") {
					//this stopped working. need to debug
					//i.e video is not showing on webapp
					c.Join(new_session, nil)
					defer e.DelClient(c)
					var producer *client.GSTProducer
					log.Infof("starting new gst producer %v", file)
					if file == "test" {
						producer = client.NewGSTProducer("video", "")
					} else {
						producer = client.NewGSTProducer("screen", file)
					}

					t, _ := c.GetPubTransport().GetPeerConnection().AddTransceiverFromTrack(producer.AudioTrack(), webrtc.RTPTransceiverInit{
						Direction: webrtc.RTPTransceiverDirectionSendonly,
					})
					defer c.UnPublish(t)

					t2, _ := c.GetPubTransport().GetPeerConnection().AddTransceiverFromTrack(producer.VideoTrack(), webrtc.RTPTransceiverInit{
						Direction: webrtc.RTPTransceiverDirectionSendonly,
					})
					defer c.UnPublish(t2)
					c.OnNegotiationNeeded()

					go func() {
						ticker := time.NewTicker(3 * time.Second)
						defer ticker.Stop()
						for {
							select {
							case <-cancel:
								return
							case <-ticker.C:
								clients, totalRecvBW, _ := e.GetStat(3)
								totalSendBW := producer.GetSendBandwidth(3)
								info := fmt.Sprintf("Clients: %d\n", clients)
								info += fmt.Sprintf("RecvBandWidth: %d KB/s\n", totalRecvBW)
								info += fmt.Sprintf("SendBandWidth: %d KB/s\n", totalSendBW)
								log.Infof(info)
							}
						}
					}()

					producer.Start()
					defer producer.Stop()

					defer func() {
						log.Infof("closing tracks")
					}()

					log.Infof("tracks published")

				} else {
					c.Join(new_session, nil)
					defer e.DelClient(c)
					c.PublishWebm(file, video, audio)
					util.GetEngineStats(e, cancel)
				}
				c.Simulcast(simulcast)

			case "sub":
				cid := fmt.Sprintf("%s_sub_%d_%s", new_session, i, cuid.New())
				log.Errorf("AddClient session=%v clientid=%v addr=%v", new_session, cid, sfu_host)
				c, err := sdk.NewClient(e, sfu_host, cid)
				if err != nil {
					log.Errorf("%v", err)
					break
				}
				handleDataChannel(c, i, cid)
				// config := sdk.NewJoinConfig().SetNoPublish() //TODO bug raise wait for fix
				c.Join(new_session, nil)
				defer e.DelClient(c)
				c.Simulcast(simulcast)
				util.GetEngineStats(e, cancel)
			default:
				log.Infof("invalid role! should be pubsub/sub")
			}

			select {
			case <-timer.C:
				util.CloseAction()
				return
			case <-cancel:
				util.CloseAction()
				log.Infof("cancel called on load test")
				return
			}
		}(i, session)
		time.Sleep(time.Millisecond * time.Duration(cycle))
	}
	return e
}

func handleDataChannel(c *sdk.Client, i int, cid string) {
	//specific to my frontend app not needed as such
	c.OnDataChannel = func(dc *webrtc.DataChannel) {

		dc.OnMessage(func(msg webrtc.DataChannelMessage) {
			var m message
			json.Unmarshal(msg.Data, &m)
			// log.Infof("m %v", m)

			t := c.GetPubTransport().GetPeerConnection().GetSenders()
			streamid := t[0].Track().StreamID()

			if m.Action == "profile" {
				p := profile{
					Name:  fmt.Sprintf("loadtest-%v", i),
					Email: fmt.Sprintf("loadtest-%v@gmail.com", i),
				}
				m := message{
					Action:     "reply-profile",
					Data:       p,
					Id:         cid,
					Streamid:   streamid,
					IsHost:     false,
					CanPublish: false,
				}
				b, _ := json.Marshal(m)
				dc.Send(b)
			}
		})
		dc.OnOpen(func() {
			// log.Infof("dc open open")
			t := c.GetPubTransport().GetPeerConnection().GetSenders()
			streamid := t[0].Track().StreamID()
			p := profile{
				Name:  fmt.Sprintf("loadtest-%v", i),
				Email: fmt.Sprintf("loadtest-%v@gmail.com", i),
			}
			m := message{
				Action:     "profile",
				Data:       p,
				Id:         cid,
				Streamid:   streamid,
				IsHost:     false,
				CanPublish: false,
			}
			b, _ := json.Marshal(m)
			dc.Send(b)
		})
	}
}
