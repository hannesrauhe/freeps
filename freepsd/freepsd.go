package main

import (
	"context"
	"flag"
	"net/url"
	"os"

	logrus "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/freepslisten"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool

type loggingConfig struct {
	Level            logrus.Level
	DisableTimestamp bool
	DisableQuote     bool
}

func configureLogging(cr *utils.ConfigReader, logger *logrus.Logger) {
	loggingConfig := loggingConfig{Level: logrus.InfoLevel, DisableTimestamp: false, DisableQuote: false}
	cr.ReadSectionWithDefaults("logging", &loggingConfig)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: loggingConfig.DisableTimestamp,
		DisableQuote:     loggingConfig.DisableQuote,
	})
	logger.SetLevel(loggingConfig.Level)
}

func main() {
	var configpath, fn, mod, argstring string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&mod, "m", "", "Specify mod to execute directly without starting rest server")
	flag.StringVar(&fn, "f", "", "Specify function to execute in mod")
	flag.StringVar(&argstring, "a", "", "Specify arguments to function as urlencoded string")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Parse()

	logger := logrus.StandardLogger()
	running := true
	for running {
		cr, err := utils.NewConfigReader(logger.WithField("component", "config"), configpath)

		if verbose {
			logger.SetLevel(logrus.DebugLevel)
		}

		if err != nil {
			logger.Fatal(err)
		}
		configureLogging(cr, logger)

		logger.Debug("Loading graph engine")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// doer := freepsdo.NewTemplateMod(cr)
		ge := freepsgraph.NewGraphEngine(cr, cancel)

		if mod != "" {
			//TODO(HR): set logger to direct stdout output
			args, _ := url.ParseQuery(argstring)
			output := ge.ExecuteOperatorByName(logger, mod, fn, utils.URLArgsToMap(args), freepsgraph.MakeEmptyOutput())
			output.WriteTo(os.Stdout)
			return
		}

		logger.Printf("Starting Listeners")
		http := freepslisten.NewFreepsHttp(cr, ge)

		// rest := freepslisten.NewRestEndpoint(cr, doer, cancel)
		mqtt := freepslisten.NewMqttSubscriber(logger, cr, ge)
		telg := freepslisten.NewTelegramBot(cr, ge, cancel)

		select {
		case <-ctx.Done():
			// Shutdown the server when the context is canceled
			// rest.Shutdown(ctx)
			if mqtt != nil {
				mqtt.Shutdown()
			}
			if telg != nil {
				telg.Shutdown(ctx)
			}
			http.Shutdown(ctx)
		}
		running = ge.ReloadRequested()
		logger.Printf("Stopping Listeners")
	}
}
