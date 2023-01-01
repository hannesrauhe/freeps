package main

import (
	"bufio"
	"context"
	"flag"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	logrus "github.com/sirupsen/logrus"

	freepsexec "github.com/hannesrauhe/freeps/connectors/exec"
	"github.com/hannesrauhe/freeps/connectors/freepsflux"
	"github.com/hannesrauhe/freeps/connectors/mqtt"
	"github.com/hannesrauhe/freeps/connectors/postgres"
	"github.com/hannesrauhe/freeps/connectors/telegram"
	"github.com/hannesrauhe/freeps/connectors/wled"
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
	var configpath, fn, mod, argstring, input string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&mod, "m", "", "Specify mod to execute directly without starting rest server")
	flag.StringVar(&fn, "f", "", "Specify function to execute in mod")
	flag.StringVar(&argstring, "a", "", "Specify arguments to function as urlencoded string")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.StringVar(&input, "i", "", "input file, use \"-\" to read from stdin")

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

		ge := freepsgraph.NewGraphEngine(cr, cancel)
		//TODO(HR): load operators from config?
		ge.AddOperator(mqtt.NewMQTTOp(cr))
		ge.AddOperator(telegram.NewTelegramOp(cr))
		ge.AddOperator(freepsflux.NewFluxMod(cr))
		ge.AddOperator(postgres.NewPostgresOp())
		ge.AddOperator(&wled.OpWLED{})
		freepsexec.AddExecOperators(cr, ge)

		ph, err := postgres.NewPostgressHook(cr)
		if err != nil {
			logger.Fatal(err)
		}
		ge.AddHook(ph)

		if mod != "" {
			args, _ := url.ParseQuery(argstring)
			oio := freepsgraph.MakeEmptyOutput()

			if input == "-" {
				scanner := bufio.NewScanner(os.Stdin)
				b := []byte{}
				for scanner.Scan() {
					b = append(b, scanner.Bytes()...)
				}
				oio = freepsgraph.MakeByteOutput(b)
			} else if input != "" {
				content, err := ioutil.ReadFile(input)
				if err != nil {
					log.Fatal(err)
				}
				oio = freepsgraph.MakeByteOutput(content)
			}
			output := ge.ExecuteOperatorByName(utils.NewContext(logger), mod, fn, utils.URLArgsToMap(args), oio)
			output.WriteTo(os.Stdout)
			return
		}

		logger.Printf("Starting Listeners")
		http := freepslisten.NewFreepsHttp(cr, ge)
		mqtt := mqtt.GetInstance()
		if err := mqtt.Init(logger, cr, ge); err != nil {
			logger.Errorf("MQTT not started: %v", err)
		}
		telg := telegram.NewTelegramBot(cr, ge, cancel)

		select {
		case <-ctx.Done():
			// Shutdown the server when the context is canceled
			mqtt.Shutdown()
			telg.Shutdown(ctx)
			http.Shutdown(ctx)
		}
		running = ge.ReloadRequested()
		ge.Shutdown(utils.NewContext(logger))
		logger.Printf("Stopping Listeners")
	}
}
