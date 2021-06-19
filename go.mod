module github.com/manishiitg/actions

go 1.16

replace github.com/pion/ion-sdk-go => ./ion-sdk-go

require (
	cloud.google.com/go/storage v1.15.0
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.7.2
	github.com/go-playground/validator/v10 v10.6.1 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/lucsky/cuid v1.2.0
	github.com/mattn/go-isatty v0.0.13 // indirect
	github.com/petar/GoLLRB v0.0.0-20210522233825-ae3b015fd3e9 // indirect
	github.com/pion/ion-avp v1.8.4
	github.com/pion/ion-log v1.2.0
	github.com/pion/ion-sdk-go v0.5.1-0.20210523062803-1e8126477a4c
	github.com/pion/rtcp v1.2.6
	github.com/pion/rtp v1.6.5
	github.com/pion/webrtc/v3 v3.0.29
	github.com/shirou/gopsutil/v3 v3.21.4
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	github.com/ugorji/go v1.2.6 // indirect
	go.etcd.io/etcd/client/v3 v3.5.0
	go.uber.org/atomic v1.8.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a // indirect
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	golang.org/x/term v0.0.0-20210503060354-a79de5458b56 // indirect
	google.golang.org/genproto v0.0.0-20210617175327-b9e0b3197ced // indirect
	google.golang.org/grpc v1.38.0
)
