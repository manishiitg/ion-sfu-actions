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
								fmt.Println("sfu host host", sfu_host, "for session", new_session, "got new session", response.Session)
								notify <- sfu_host + "=" + response.Session //TODO this string is a temporary solution should be a strcut
							} else {
								fmt.Println("sfu host host", sfu_host, "for session", new_session)
								notify <- sfu_host
							}
							close(done)
						}
					}
				}
			}
		}
	} else {
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
