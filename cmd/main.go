package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/handelsblattgroup/statping/database"
	"github.com/handelsblattgroup/statping/handlers"
	"github.com/handelsblattgroup/statping/notifiers"
	"github.com/handelsblattgroup/statping/source"
	"github.com/handelsblattgroup/statping/types/configs"
	"github.com/handelsblattgroup/statping/types/core"
	"github.com/handelsblattgroup/statping/types/metrics"
	"github.com/handelsblattgroup/statping/types/services"
	"github.com/handelsblattgroup/statping/utils"
)

var (
	// VERSION stores the current version of Statping
	VERSION string
	// COMMIT stores the git commit hash for this version of Statping
	COMMIT  string
	log     = utils.Log.WithField("type", "cmd")
	confgs  *configs.DbConfig
	stopped chan bool
)

func init() {
	stopped = make(chan bool, 1)
	core.New(VERSION, COMMIT)
	utils.InitEnvs()
	utils.Params.Set("VERSION", VERSION)
	utils.Params.Set("COMMIT", COMMIT)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(assetsCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(sassCmd)
	rootCmd.AddCommand(onceCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(systemctlCmd)
	rootCmd.AddCommand(resetCmd)

	parseFlags(rootCmd)
}

// exit will return an error and return an exit code 1 due to this error
func exit(err error) {
	utils.SentryErr(err)
	log.Fatalln(err)
	os.Exit(1)
}

// Close will gracefully stop the database connection, and log file
func Close() {
	utils.CloseLogs()
	confgs.Close()
	fmt.Println("Shutting down Statping")
	//TODO: handlers.StopHTTPServer(nil)
}

// main will run the Statping application
func main() {
	go Execute()
	<-stopped
	Close()
}

// main will run the Statping application
func start() {
	go sigterm()
	var err error
	if err := source.Assets(); err != nil {
		exit(err)
	}

	utils.VerboseMode = verboseMode

	if err := utils.InitLogs(); err != nil {
		log.Errorf("Statping Log Error: %v\n", err)
	}

	log.Info(fmt.Sprintf("Starting Statping v%s", VERSION))

	utils.Params.Set("SERVER_IP", ipAddress)
	utils.Params.Set("SERVER_PORT", port)
	utils.Params.Set("MAINTENANCE_SERVER_IP", maintenanceIpAddress)
	utils.Params.Set("MAINTENANCE_SERVER_PORT", maintenancePort)
	utils.Params.Set("MAINTENANCE_SERVER_ENABLED", maintenanceServerEnabled)

	confgs, err = configs.LoadConfigs(configFile)
	if err != nil {
		log.Infoln("Starting in Setup Mode")
		handlers.RunHTTPServer()
	}

	if err = configs.ConnectConfigs(confgs, true); err != nil {
		exit(err)
	}

	if err = confgs.ResetCore(); err != nil {
		exit(err)
	}

	if err = confgs.DatabaseChanges(); err != nil {
		exit(err)
	}

	if err := confgs.MigrateDatabase(); err != nil {
		exit(err)
	}

	log.Infoln("Starting in Normal Mode")
	mainProcess()
}

// sigterm will attempt to close the database connections gracefully
func sigterm() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	stopped <- true
}

// mainProcess will initialize the Statping application and run the HTTP server
func mainProcess() {
	if err := InitApp(); err != nil {
		exit(err)
	}

	services.LoadServicesYaml()
	handlers.RunHTTPServer()
	return
}

// InitApp will start the Statping instance with a valid database connection
// This function will gather all services in database, add/init Notifiers,
// and start the database cleanup routine
func InitApp() error {
	// fetch Core row information about this instance.
	if _, err := core.Select(); err != nil {
		return err
	}
	// init Sentry error monitoring (its useful)
	utils.SentryInit(core.App.AllowReports.Bool)
	// init prometheus metrics
	metrics.InitMetrics()
	// connect each notifier, added them into database if needed
	notifiers.InitNotifiers()
	// select all services in database and store services in a mapping of Service pointers
	if _, err := services.SelectAllServices(true); err != nil {
		return err
	}
	// start routines for each service checking process
	services.CheckServices()
	// start routine to delete old records (failures, hits)
	go database.Maintenance()
	core.App.Setup = true
	core.App.Started = utils.Now()
	return nil
}
