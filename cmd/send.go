package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	send "github.com/manishiitg/actions/gstreamer-send"
	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "send video to sfu via gstreamer",
	RunE:  gstreamersend,
}

var audioSrc, videoSrc string

func init() {
	streamCmd.PersistentFlags().StringVarP(&audioSrc, "audioSrc", "", "audiotestsrc", "Gstreamer audio src")
	streamCmd.PersistentFlags().StringVarP(&videoSrc, "videoSrc", "", "videotestsrc", "Gstreamer video src")
	rootCmd.AddCommand(sendCmd)
}
func gstreamersend(cmd *cobra.Command, args []string) error {
	cancel := make(chan struct{})
	send.Init(caddr, session, audioSrc, videoSrc, cancel)

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
