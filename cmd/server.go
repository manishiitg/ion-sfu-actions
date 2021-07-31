package cmd

import (
	"os"
	"os/signal"
	"syscall"

	actions "github.com/manishiitg/actions/api"
	ip "github.com/manishiitg/actions/ip"
	log "github.com/pion/ion-log"
	"github.com/spf13/cobra"
)

func getEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return ""
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start the actions server",
	RunE:  serverMain,
}

var eaddr, ipaddr, port, saddr string

func init() {
	serverCmd.PersistentFlags().StringVarP(&eaddr, "eaddr", "e", "0.0.0.0:2379", "etcd host to connect")
	serverCmd.PersistentFlags().StringVarP(&ipaddr, "ipaddr", "", "5.9.18.28", "ip address of current server")         //TODO remove this ip
	serverCmd.PersistentFlags().StringVarP(&saddr, "saddr", "", "http://5.9.18.28:4000/", "server cluster ip address") //TODO remove this ip
	serverCmd.PersistentFlags().StringVarP(&port, "port", "p", ":3050", "port of server")
	serverCmd.MarkFlagRequired("ipaddr")
	serverCmd.MarkFlagRequired("saddr")

	rootCmd.AddCommand(serverCmd)

}

func serverMain(cmd *cobra.Command, args []string) error {
	if len(ipaddr) == 0 {
		ipaddr = ip.GetIP()
	}
	if len(eaddr) == 0 || len(port) == 0 || len(saddr) == 0 {
		log.Infof("ipaddr %v, eaddr %v, port %v , saddr %v all requried", ipaddr, eaddr, port, saddr)
		os.Exit(-1)
	}

	log.Init("info")

	e, err := actions.InitEtcd(eaddr, ipaddr, port, saddr)
	if err != nil {
		log.Errorf("unable to connect to etd %v", err)
		return err
	}
	err = e.InitApi(port)

	if err != nil {
		log.Errorf("unable to start api server %v", err)
		return err
	}

	// Listen for signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Select on error channels from different modules
	for {
		select {
		case sig := <-sigs:
			log.Infof("Got signal, beginning shutdown", "signal", sig)
			os.Exit(1)
			return nil
		}
	}

	return nil
}
