package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	loadtest "github.com/manishiitg/actions/loadtest"
	"github.com/manishiitg/actions/loadtest/client/gst"
	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "loadtest",
	Short: "start the actions server",
	RunE:  loadMain,
}

var file, role, loglevel, simulcast, paddr string
var total, cycle, duration int
var create_room = -1

func init() {
	loadCmd.PersistentFlags().StringVarP(&file, "file", "f", "480p", "type of file either test 360p 480p 720p h246")
	loadCmd.PersistentFlags().IntVarP(&total, "clients", "c", 1, "Number of clients to start")
	loadCmd.PersistentFlags().IntVarP(&cycle, "cycle", "y", 1000, "Run new client cycle in ms")
	loadCmd.PersistentFlags().IntVarP(&duration, "duration", "d", 60*60, "Running duration in seconds")
	loadCmd.PersistentFlags().StringVarP(&role, "role", "r", "pubsub", "Run as pubsub/sub")
	loadCmd.PersistentFlags().IntVarP(&create_room, "create_room", "x", -1, "number of peers per room")
	rootCmd.AddCommand(loadCmd)
}

func clientThread() {

	cancel := make(chan struct{})
	go loadtest.Init(file, caddr, session, total, cycle, duration, role, create_room, -1, cancel)

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

}

func loadMain(cmd *cobra.Command, args []string) error {
	go clientThread()
	gst.MainLoop()
	return nil
}
