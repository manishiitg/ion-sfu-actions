package cmd

import (
	mirrorsfu "github.com/manishiitg/actions/mirror-sfu"
	"github.com/spf13/cobra"
)

var mirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "mirror sfu track",
	RunE:  mirrorMain,
}

var session2 string

func init() {
	mirrorCmd.PersistentFlags().StringVarP(&session2, "session2", "", "test2", "which session to mirror to")
	rootCmd.AddCommand(mirrorCmd)
}

func mirrorMain(cmd *cobra.Command, args []string) error {

	cancel := make(chan struct{})
	mirrorsfu.InitWithAddress(session, session2, caddr, cancel)

	return nil
}
