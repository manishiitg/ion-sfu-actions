package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/pion/ion-log"
	sdk "github.com/pion/ion-sdk-go"

	"github.com/shirou/gopsutil/v3/cpu"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

const LEASE_TIMEOUT = 5
const LEASE_RENEW_TIMEOUT = 1 //lease renuew timeout should be less then lese timeout always
// 1sec is better as we get to know of host load every 1sec

type etcdCoordinator struct {
	nodeIp             string
	nodePort           string
	client             *clientv3.Client
	kvc                clientv3.KV
	lease              *clientv3.LeaseGrantResponse
	mu                 sync.Mutex
	engine             *sdk.Engine
	mirrorActionCancel chan struct{}
	loadActionCancel   chan struct{}
	streamActionCancel chan struct{}
	diskActionCancel   chan struct{}
	serverIp           string
}

func InitEtcd(eaddr string, ipaddr string, port string, saddr string) (e *etcdCoordinator, err error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{eaddr},
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Errorf("unable to connect to etcd", err)
		return nil, err
	}
	log.Infof("etcd client connected", "eaddr", eaddr, "ipaddr", ipaddr, "port", port, "saddr", saddr)
	kvc := clientv3.NewKV(cli)
	etcdObj := &etcdCoordinator{
		client:   cli,
		nodeIp:   ipaddr,
		nodePort: port,
		kvc:      kvc,
		serverIp: saddr,
	}

	etcdObj.createHostLease()

	ticker := time.NewTicker(LEASE_RENEW_TIMEOUT * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				e.notifyAlive()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return etcdObj, nil
}

type Load struct {
	Cpu   float64 `json:"cpu"`
	Tasks int     `json:"tasks"`
	Ip    string  `json:"ip"`
	Port  string  `json:"port"`
}

func (e *etcdCoordinator) getHostLoad() Load {
	x, _ := cpu.Percent(time.Second, false)
	task := 0
	// log.Infof("engine %v", e.engine)
	if e.engine != nil {
		task = 1
	}
	load := Load{
		Cpu:   x[0],
		Tasks: task,
		Ip:    e.nodeIp,
		Port:  e.nodePort,
	}
	return load
}

func (e *etcdCoordinator) createHostLease() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	e.mu.Lock()
	defer e.mu.Unlock()
	// First lets create a lease for the host
	lease, err := e.client.Grant(ctx, LEASE_TIMEOUT) //10sec
	if err != nil {
		log.Errorf("error acquiring lease for session key", err)
		return
	}
	e.lease = lease
	log.Infof("Got lease", "ID", lease.ID, "TTL", lease.TTL)
	load := e.getHostLoad()
	b, _ := json.Marshal(load)
	e.kvc.Put(ctx, "action-hosts/"+e.getHostKey(), string(b), clientv3.WithLease(lease.ID))
}

func (e *etcdCoordinator) notifyAlive() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.lease != nil {
		leaseKeepAlive, err := e.client.KeepAlive(context.Background(), e.lease.ID)
		if err != nil {
			log.Errorf("error activating keepAlive for lease", err, "leaseID", e.lease.ID)
		}
		log.Debugf("leaseKeepAlive err %v", leaseKeepAlive)

		// === === see here
		go func() {
			for {
				<-leaseKeepAlive
			}
		}()
		load := e.getHostLoad()
		b, _ := json.Marshal(load)
		// log.Info("host load %v", string(b))
		resp, err := e.kvc.Put(context.Background(), "action-hosts/"+e.getHostKey(), string(b), clientv3.WithLease(e.lease.ID))
		if err != nil {
			log.Infof("resp v=%v", resp)
			log.Errorf("err", err)
			log.Infof("lease id %v", e.lease.ID)
			errstr := fmt.Sprintf("%v", err)
			log.Infof("errstr %v", errstr)
			if strings.Index(errstr, "requested lease not found") != -1 {
				log.Infof("lease not found maybe server was restarted, will get new lease")
				e.lease = nil
			}
		}
		// log.Infof("Host Alive  leaseKeepAlive %v", <-leaseKeepAlive)
	} else {
		go e.createHostLease()
	}
}

func (e *etcdCoordinator) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.client.Close()
}

func (e *etcdCoordinator) getHostKey() string {
	value := e.nodeIp + ":" + e.nodePort
	return strings.Replace(value, "::", ":", -1)
}
