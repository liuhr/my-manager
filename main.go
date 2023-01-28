package main

import (
	"flag"

	"github.com/github/my-manager/app"
	"github.com/github/my-manager/config"
	"github.com/openark/golib/log"
)

// main is the application's entry point. It will either spawn a CLI or HTTP itnerfaces.
func main() {
	configFile := flag.String("config", "", "config file name")
	verbose := flag.Bool("verbose", false, "verbose")
	debug := flag.Bool("debug", false, "debug mode (very verbose)")
	stack := flag.Bool("stack", false, "add stack trace upon error")
	flag.Parse()

	log.SetLevel(log.ERROR)
	if *verbose {
		log.SetLevel(log.INFO)
	}
	if *debug {
		log.SetLevel(log.DEBUG)
	}
	if *stack {
		log.SetPrintStackTrace(*stack)
	}

	appVersion := config.NewAppVersion()

	startText := "starting my-manager"
	if appVersion != "" {
		startText += ", version: " + appVersion
	}
	log.Info(startText)
	if len(*configFile) > 0 {
		config.ForceRead(*configFile)
	} else {
		config.Read("/etc/my-manager.conf.json",
			"conf/my-manager.conf.json",
			"my-manager.conf.json")
	}
	if config.Config.Debug {
		log.SetLevel(log.DEBUG)
	}
	config.MarkConfigurationLoaded()
	app.Http()
}
