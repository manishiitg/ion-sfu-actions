package util

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"
	"github.com/pion/webrtc/v3"
)

func GetEngine() *sdk.Engine {
	webrtcCfg := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			webrtc.ICEServer{
				URLs: []string{"stun:stun.stunprotocol.org:3478", "stun:stun.l.google.com:19302"},
			},
		},
	}

	config := sdk.Config{
		WebRTC: sdk.WebRTCTransportConfig{
			Configuration: webrtcCfg,
		},
	}
	// new sdk engine
	return sdk.NewEngine(config)
}

type HostResponse struct {
	Host    string
	Session string
	Status  string
	Publish bool //not using this as of now as purpose of load test will fail
}

func GetHost(addr string, new_session string, notify chan string, cancel <-chan struct{}, role string, capacity int) {

	// notify <- "0.0.0.0:50052"

	if strings.Contains(addr, "https://") || strings.Contains(addr, "http://") {
		// this is cooridinator hacky assumption

		done := make(chan struct{})
		for {
			select {
			case <-done:
				log.Debugf("get host done!")
				return
			case <-cancel:
				log.Debugf("get host cancelled, cleanup")
				return
			default:
				log.Debugf("getting sfu from %v for capacity %v", addr, capacity)
				var resp *http.Response
				var err error
				if capacity == -1 {
					resp, err = http.Get(addr + "session/" + new_session + "?role=" + role)
				} else {
					resp, err = http.Get(addr + "session/" + new_session + "?role=" + role + "&capacity=" + strconv.Itoa(capacity))
				}
				if err != nil {
					log.Errorf("%v", err)
					time.Sleep(10 * time.Second)
				} else {
					body, err2 := ioutil.ReadAll(resp.Body)
					if err2 != nil {
						log.Errorf("%v", err)
						time.Sleep(10 * time.Second)
					} else {
						var response HostResponse
						err = json.Unmarshal(body, &response)
						if err != nil {
							log.Errorf("error parsing host response", err)
						}
						sfu_host := response.Host
						log.Debugf("response %v", response, " status %v", response.Status)
						if sfu_host == "NO_HOSTS_RETRY" {
							log.Debugf("waiting for host to get ready!")
							time.Sleep(2 * time.Second)
						} else if sfu_host == "SERVER_LOAD" {
							log.Debugf("server is underload need to wait before joining call!")
							time.Sleep(2 * time.Second)
						} else if len(sfu_host) == 0 {
							log.Debugf("host not found")
							time.Sleep(2 * time.Second)
						} else {
							sfu_host = strings.Replace(sfu_host, "700", "5005", -1) //TODO need a proper solution
							sfu_host = strings.Replace(sfu_host, "\"", "", -1)
							if len(response.Session) > 0 {
								log.Infof("sfu host host", sfu_host, "for session", new_session, "got new session", response.Session)
								notify <- sfu_host + "=" + response.Session //TODO this string is a temporary solution should be a strcut
							} else {
								log.Infof("sfu host host", sfu_host, "for session", new_session)
								notify <- sfu_host
							}
							close(done)
						}
					}
				}
			}
		}
	} else {
		log.Infof("direct sfu %v", addr)
		notify <- addr
	}

}

func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func GetEngineStats(e *sdk.Engine, cancel <-chan struct{}) {
	go e.Stats(3, cancel)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-cancel:
				return
			case <-ticker.C:
				clients, totalRecvBW, totalSendBW := e.GetStat()
				info := fmt.Sprintf("Clients: %d\n", clients)
				info += fmt.Sprintf("RecvBandWidth: %d KB/s\n", totalRecvBW)
				info += fmt.Sprintf("SendBandWidth: %d KB/s\n", totalSendBW)
				log.Infof(info)
				UpdateActionProgress(info)
			}
		}
	}()
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

func HandleDataChannel(c *sdk.Client, name string, i int, cid string) {
	//specific to my frontend app not needed as such
	c.OnDataChannel = func(dc *webrtc.DataChannel) {
		HandleData(c, dc, name, i, cid)
	}
}

func HandleProfileMsg(c *sdk.Client, dc *webrtc.DataChannel, msg webrtc.DataChannelMessage, name string, i int, cid string) {
	var m message
	json.Unmarshal(msg.Data, &m)
	// log.Infof("m %v", m)

	if m.Action == "profile" {
		log.Infof("is profile message")
		t := c.GetPubTransport().GetPeerConnection().GetSenders()
		if len(t) > 0 {
			streamid := t[0].Track().StreamID()
			p := profile{
				Name:  fmt.Sprintf("%v-%v", name, i),
				Email: fmt.Sprintf("%v-%v@gmail.com", i),
			}
			m := message{
				Action:     "reply-profile",
				Data:       p,
				Id:         cid,
				Streamid:   streamid,
				IsHost:     false,
				CanPublish: true,
			}
			log.Infof("profile reply %v", m)
			b, _ := json.Marshal(m)
			dc.Send(b)
		} else {
			log.Infof("unable to handle profile msg")
		}
	}
}

func HandleData(c *sdk.Client, dc *webrtc.DataChannel, name string, i int, cid string) {
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		HandleProfileMsg(c, dc, msg, name, i, cid)
	})
	dc.OnOpen(func() {
		// log.Infof("dc open open")
		t := c.GetPubTransport().GetPeerConnection().GetSenders()
		if len(t) > 0 {
			streamid := t[0].Track().StreamID()
			p := profile{
				Name:  fmt.Sprintf("%v-%v", name, i),
				Email: fmt.Sprintf("%v-%v@gmail.com", name, i),
			}
			m := message{
				Action:     "profile",
				Data:       p,
				Id:         cid,
				Streamid:   streamid,
				IsHost:     false,
				CanPublish: true,
			}
			b, _ := json.Marshal(m)
			dc.Send(b)
		}
	})
}

type flipmsg struct {
	Flip      bool   `json:"flip"`
	InputType string `json:"inputType"`
	Id        string `json:"id"`
	Streamid  string `json:"streamid"`
}

func SendFlip(dc *webrtc.DataChannel, id, streamid string) {
	m := flipmsg{
		Flip:      true,
		InputType: "mirror",
		Id:        id,
		Streamid:  streamid,
	}
	b, _ := json.Marshal(m)
	dc.Send(b)
}

func SendData(c *sdk.Client, name string, i int, cid string, sendflip bool) {

	dc, _ := c.CreateDataChannel("data")
	if dc.ReadyState().String() == "open" {
		sendDataOpen(c, dc, name, i, cid, sendflip)
	} else {
		dc.OnOpen(func() {
			sendDataOpen(c, dc, name, i, cid, sendflip)
		})
	}
}
func sendDataOpen(c *sdk.Client, dc *webrtc.DataChannel, name string, i int, cid string, sendflip bool) {
	t := c.GetPubTransport().GetPeerConnection().GetSenders()
	streamid := t[0].Track().StreamID()
	p := profile{
		Name:  fmt.Sprintf("%v-%v", name, i),
		Email: fmt.Sprintf("%v-%v@gmail.com", name, i),
	}
	m := message{
		Action:     "profile",
		Data:       p,
		Id:         cid,
		Streamid:   streamid,
		IsHost:     false,
		CanPublish: true,
	}
	b, _ := json.Marshal(m)
	dc.Send(b)
	if sendflip {
		SendFlip(dc, cid, streamid)
		time.AfterFunc(time.Second*5, func() {
			SendFlip(dc, cid, streamid)
		})

	}
}
