package main

import (
	"github.com/handelsblattgroup/statping/utils"
	"github.com/spf13/cobra"
)

var (
	ipAddress                string
	configFile               string
	verboseMode              int
	port                     int
	maintenanceIpAddress     string
	maintenancePort          int
	maintenanceServerEnabled bool
)

func parseFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&ipAddress, "ip", "s", "0.0.0.0", "server run on host")
	utils.Params.BindPFlag("ip", cmd.PersistentFlags().Lookup("ip"))

	cmd.PersistentFlags().IntVarP(&port, "port", "p", 8080, "server port")
	utils.Params.BindPFlag("port", cmd.PersistentFlags().Lookup("port"))

	cmd.PersistentFlags().IntVarP(&verboseMode, "verbose", "v", 2, "verbose logging")
	utils.Params.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose"))

	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", utils.Directory+"/config.yml", "path to config.yml file")
	utils.Params.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))

	cmd.PersistentFlags().StringVar(&maintenanceIpAddress, "maintenance-ip", "0.0.0.0", "maintenance server run on host")
	utils.Params.BindPFlag("maintenance-ip", cmd.PersistentFlags().Lookup("maintenance-ip"))

	cmd.PersistentFlags().IntVar(&maintenancePort, "maintenance-port", 8081, "maintenance server port")
	utils.Params.BindPFlag("maintenance-port", cmd.PersistentFlags().Lookup("maintenance-port"))

	cmd.PersistentFlags().BoolVar(&maintenanceServerEnabled, "maintenance-enabled", false, "enable maintenance server")
	utils.Params.BindPFlag("maintenance-enabled", cmd.PersistentFlags().Lookup("maintenance-enabled"))
}
