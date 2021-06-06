package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	rtmptotrack "github.com/manishiitg/actions/rtmptotrack"
	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
)

var rtmpCmd = &cobra.Command{
	Use:   "rtmptotrack",
	Short: "listen to rtmp and publish track",
	RunE:  loadrtmp,
}

var rtmpInput string

func init() {
	streamCmd.PersistentFlags().StringVarP(&rtmpInput, "irtmp", "", "demo", "RTMP URL TO read From")
	rootCmd.AddCommand(rtmpCmd)
}
func loadrtmp(cmd *cobra.Command, args []string) error {
	cancel := make(chan struct{})
	rtmptotrack.Init(session, caddr, rtmpInput, cancel)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sig := <-sigs:
			log.Infof("Got signal, beginning shutdown", "signal", sig)
			close(cancel)
			time.AfterFunc(1*time.Second, func() {
				os.Exit(1)
			})
		}
	}
}
