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

	"github.com/hannesrauhe/freeps/base"
	freepsbluetooth "github.com/hannesrauhe/freeps/connectors/bluetooth"
	freepsexec "github.com/hannesrauhe/freeps/connectors/exec"
	"github.com/hannesrauhe/freeps/connectors/freepsflux"
	"github.com/hannesrauhe/freeps/connectors/fritz"
	"github.com/hannesrauhe/freeps/connectors/mqtt"
	"github.com/hannesrauhe/freeps/connectors/muteme"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/connectors/telegram"
	"github.com/hannesrauhe/freeps/connectors/ui"
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
	logger.Infof("Freeps %v", utils.BuildFullVersion())
	running := true
	for running {
		cr, err := utils.NewConfigReader(logger.WithField("component", "config"), configpath)
		if err != nil {
			logger.Fatal(err)
		}
		configureLogging(cr, logger)
		if verbose {
			logger.SetLevel(logrus.DebugLevel)
		}

		if verbose {
			logger.SetLevel(logrus.DebugLevel)
		}

		_, err = utils.GetTempDir()
		if err != nil {
			logger.Fatal("Temp dir creation failed: ", err.Error())
		}
		defer utils.DeleteTempDir()

		logger.Debug("Loading graph engine")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ge := freepsgraph.NewGraphEngine(cr, cancel)

		mm, err := muteme.NewMuteMe(logger, cr, ge)
		if err != nil {
			logger.Errorf("MuteMe not started: %v", err)
		} else {
			ge.AddOperator(muteme.NewMuteMeOp(mm))
		}

		fbt, err := freepsbluetooth.NewBTWatcher(logger, cr, ge)
		if err != nil {
			logger.Errorf("FreepsBT not started: %v", err)
		}

		//TODO(HR): load operators from config?
		ge.AddOperator(freepsstore.NewOpStore(cr)) //needs to be first for now
		ge.AddOperator(mqtt.NewMQTTOp(cr))
		ge.AddOperator(telegram.NewTelegramOp(cr))
		ge.AddOperator(freepsflux.NewFluxMod(cr))
		ge.AddOperator(wled.NewWLEDOp(cr))
		ge.AddOperator(ui.NewHTMLUI(cr, ge))
		ge.AddOperator(fritz.NewOpFritz(cr))
		freepsexec.AddExecOperators(cr, ge)

		sh, err := freepsstore.NewStoreHook(cr)
		if err != nil {
			logger.Errorf("Store hook not available: %v", err.Error())
		} else {
			ge.AddHook(sh)
		}

		if err := ge.LoadEmbeddedGraphs(); err != nil {
			logger.Fatal(err)
		}

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
			output := ge.ExecuteOperatorByName(base.NewContext(logger), mod, fn, utils.URLArgsToMap(args), oio)
			output.WriteTo(os.Stdout)
			return
		}

		logger.Printf("Starting Listeners")
		http := freepslisten.NewFreepsHttp(cr, ge)
		m := mqtt.GetInstance()
		if err := m.Init(logger, cr, ge); err != nil {
			logger.Errorf("MQTT not started: %v", err)
		} else {
			h, _ := mqtt.NewMQTTHook(cr)
			ge.AddHook(h)
		}
		telg := telegram.NewTelegramBot(cr, ge, cancel)
		mm.StartListening()

		select {
		case <-ctx.Done():
			// Shutdown the server when the context is canceled
			m.Shutdown()
			telg.Shutdown(context.TODO())
			http.Shutdown(context.TODO())
			mm.Shutdown()
			fbt.Shutdown()
		}
		running = ge.ReloadRequested()
		ge.Shutdown(base.NewContext(logger))
		logger.Printf("Stopping Listeners")
	}
}
