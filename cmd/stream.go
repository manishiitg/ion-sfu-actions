package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	tracktortp "github.com/manishiitg/actions/tracktortp"
	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
)

var streamCmd = &cobra.Command{
	Use:   "stream",
	Short: "stream rtmp via ffmpeg",
	RunE:  loadstream,
}

var rtmp string

func init() {
	streamCmd.PersistentFlags().StringVarP(&rtmp, "rtmp", "", "", "RTMP URL to publish stream")
	rootCmd.AddCommand(streamCmd)
}

func loadstream(cmd *cobra.Command, args []string) error {
	cancel := make(chan struct{})
	tracktortp.Init(session, caddr, rtmp, cancel)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case sig := <-sigs:
			log.Infof("Got signal, beginning shutdown", "signal", sig)
			close(cancel)
			time.AfterFunc(2*time.Second, func() {
				os.Exit(1)
			})
		}
	}

	return nil
}
