package ip

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

type SimpleHTTP struct {
	// The list of endpoints to query. If empty, a default list will
	// be used:
	//
	// - https://api.ipify.org
	// - https://myip.addr.space
	// - https://ifconfig.me
	// - https://icanhazip.com
	// - https://ident.me
	// - https://bot.whatismyipaddress.com
	// - https://ipecho.net/plain
	Endpoints []string `json:"endpoints,omitempty"`
}

func GetIP() string {
	var sh SimpleHTTP
	sh.provision()
	currentIPs, err := sh.getIPs(context.Background())
	if len(currentIPs) == 0 {
		err = fmt.Errorf("no IP addresses returned")
	}
	if err == nil {
		currentIPs = removeDuplicateIPs(currentIPs)
		for _, ip := range currentIPs {
			fmt.Println("ip found", ip.String())
			return ip.String()
		}
	}
	return ""
}

// Provision sets up the module.
func (sh *SimpleHTTP) provision() {
	if len(sh.Endpoints) == 0 {
		sh.Endpoints = defaultHTTPIPServices
	}
}

// GetIPs gets the public addresses of this machine.
func (sh SimpleHTTP) getIPs(ctx context.Context) ([]net.IP, error) {
	ipv4Client := sh.makeClient("tcp4")

	var ips []net.IP
	for _, endpoint := range sh.Endpoints {
		ipv4, err := sh.lookupIP(ctx, ipv4Client, endpoint)
		if err != nil {
			fmt.Printf("IPv4 lookup failed %v %v", endpoint, err)
		} else if !ipListContains(ips, ipv4) {
			ips = append(ips, ipv4)
		}

		// use first successful service
		if len(ips) > 0 {
			break
		}
	}
	fmt.Printf("ips found %v", ips)

	return ips, nil
}

// makeClient makes an HTTP client that forces use of the specified network type (e.g. "tcp6").
func (SimpleHTTP) makeClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, _, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, address)
			},
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func (SimpleHTTP) lookupIP(ctx context.Context, client *http.Client, endpoint string) (net.IP, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: server response was: %d %s", endpoint, resp.StatusCode, resp.Status)
	}

	ipASCII, err := ioutil.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return nil, err
	}
	ipStr := strings.TrimSpace(string(ipASCII))

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("%s: invalid IP address: %s", endpoint, ipStr)
	}

	return ip, nil
}

var defaultHTTPIPServices = []string{
	"https://api.ipify.org",
	"https://myip.addr.space",
	"https://ifconfig.me",
	"https://icanhazip.com",
	"https://ident.me",
	"https://bot.whatismyipaddress.com",
	"https://ipecho.net/plain",
}

// ipListContains returns true if list contains ip; false otherwise.
func ipListContains(list []net.IP, ip net.IP) bool {
	for _, ipInList := range list {
		if ipInList.Equal(ip) {
			return true
		}
	}
	return false
}

// removeDuplicateIPs returns ips without duplicates.
func removeDuplicateIPs(ips []net.IP) []net.IP {
	var clean []net.IP
	for _, ip := range ips {
		if !ipListContains(clean, ip) {
			clean = append(clean, ip)
		}
	}
	return clean
}
