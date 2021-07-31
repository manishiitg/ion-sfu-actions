package cmd

import (
	"fmt"

	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var session, caddr, sfuaddr string

var rootCmd = &cobra.Command{
	Use:   "action",
	Short: "action is a collection of small utility to interface with ion-sfu using ion-sdk-go",
	Run:   func(*cobra.Command, []string) {},
}

func init() {
	log.Init("info")
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&caddr, "caddr", "g", "", "SFU Coordinator")
	rootCmd.PersistentFlags().StringVarP(&sfuaddr, "sfuaddr", "", "https://sfu.excellencetechnologies.info/", "Direct SFU URL")
	rootCmd.PersistentFlags().StringVarP(&session, "session", "s", "test", "join session name")

	if len(sfuaddr) != 0 {
		log.Infof("Using direct SFU instead of Coordinator")
		caddr = sfuaddr
	}

}

func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
