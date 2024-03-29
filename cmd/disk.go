package cmd

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/manishiitg/actions/loadtest/client/gst"
	tasktodisk "github.com/manishiitg/actions/tracktodisk"
	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
)

var diskCmd = &cobra.Command{
	Use:   "savetodisk",
	Short: "connect to sfu and save track to disk",
	RunE:  diskMain,
}

var storage string
var filename string
var filetype string

func init() {
	diskCmd.PersistentFlags().StringVarP(&storage, "storage", "", "local", "where to store the file on local or cloud")
	diskCmd.PersistentFlags().StringVarP(&filename, "filename", "", "sample", "name of file")
	diskCmd.PersistentFlags().StringVarP(&filetype, "filetype", "", "webm", "filetype either webm or mp4")
	rootCmd.AddCommand(diskCmd)
}

func compositeThread() {
	cancel := make(chan struct{})
	tasktodisk.Init(caddr, session, filetype, filename, storage, cancel)

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

func diskMain(cmd *cobra.Command, args []string) error {
	runtime.LockOSThread()
	go compositeThread()
	gst.MainLoop()
	return nil
}
