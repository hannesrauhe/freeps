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
	"github.com/hannesrauhe/freeps/connectors/chaosimradio"
	freepsexec "github.com/hannesrauhe/freeps/connectors/exec"
	"github.com/hannesrauhe/freeps/connectors/freepsflux"
	"github.com/hannesrauhe/freeps/connectors/fritz"
	freepshttp "github.com/hannesrauhe/freeps/connectors/http"
	"github.com/hannesrauhe/freeps/connectors/mqtt"
	"github.com/hannesrauhe/freeps/connectors/muteme"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/connectors/telegram"
	"github.com/hannesrauhe/freeps/connectors/ui"
	"github.com/hannesrauhe/freeps/connectors/wled"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool
var configpath, fn, operator, argstring, input string

type loggingConfig struct {
	Level            logrus.Level
	DisableTimestamp bool
	DisableQuote     bool
}

func configureLogging(cr *utils.ConfigReader, logger *logrus.Logger) {
	level := logrus.InfoLevel
	if verbose {
		level = logrus.DebugLevel
	}
	loggingConfig := loggingConfig{Level: level, DisableTimestamp: false, DisableQuote: false}
	cr.ReadSectionWithDefaults("logging", &loggingConfig)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: loggingConfig.DisableTimestamp,
		DisableQuote:     loggingConfig.DisableQuote,
	})
	if !verbose {
		level = loggingConfig.Level
	}
	logger.SetLevel(level)
}

func mainLoop() bool {
	// keep this here so the operators are re-created on reload
	availableOperators := []base.FreepsOperator{
		&freepsbluetooth.Bluetooth{},
		&muteme.MuteMe{},
		&freepsflux.OperatorFlux{},
		&freepsgraph.OpUtils{},
		&freepsgraph.OpRegexp{},
		&freepsgraph.OpCurl{},
		&chaosimradio.OpCiR{},
	}

	logger := logrus.StandardLogger()
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	}
	logger.Infof("Freeps %v", utils.BuildFullVersion())

	cr, err := utils.NewConfigReader(logger.WithField("component", "config"), configpath)
	if err != nil {
		logger.Fatal(err)
	}
	configureLogging(cr, logger)

	initCtx := base.NewContext(logger.WithField("phase", "init"))

	_, err = utils.GetTempDir()
	if err != nil {
		logger.Fatal("Temp dir creation failed: ", err.Error())
	}
	defer utils.DeleteTempDir()

	logger.Debug("Loading graph engine")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ge := freepsgraph.NewGraphEngine(cr, cancel)
	ge.AddOperator(freepsstore.NewOpStore(cr)) //needs to be first for now
	for _, op := range availableOperators {
		// this will automatically skip operators that are not enabled in the config
		ge.AddOperator(base.MakeFreepsOperator(op, cr, initCtx))
	}
	ge.AddOperator(mqtt.NewMQTTOp(cr))
	ge.AddOperator(telegram.NewTelegramOp(cr))
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

	if operator != "" {
		args, _ := url.ParseQuery(argstring)
		oio := base.MakeEmptyOutput()

		if input == "-" {
			scanner := bufio.NewScanner(os.Stdin)
			b := []byte{}
			for scanner.Scan() {
				b = append(b, scanner.Bytes()...)
			}
			oio = base.MakeByteOutput(b)
		} else if input != "" {
			content, err := ioutil.ReadFile(input)
			if err != nil {
				log.Fatal(err)
			}
			oio = base.MakeByteOutput(content)
		}
		output := ge.ExecuteOperatorByName(base.NewContext(logger), operator, fn, utils.URLArgsToMap(args), oio)
		output.WriteTo(os.Stdout)
		return false
	}

	logger.Infof("Starting Listeners")
	mm, err := muteme.NewMuteMe()
	if err != nil {
		logger.Errorf("MuteMe not started: %v", err)
	}

	http := freepshttp.NewFreepsHttp(cr, ge)
	m := mqtt.GetInstance()
	if err := m.Init(logger, cr, ge); err != nil {
		logger.Errorf("MQTT not started: %v", err)
	} else {
		h, _ := mqtt.NewMQTTHook(cr)
		ge.AddHook(h)
	}
	fbt, err := freepsbluetooth.NewBTWatcher(logger, cr, ge)
	if err != nil {
		logger.Errorf("FreepsBT not started: %v", err)
	} else if fbt != nil {
		ge.AddHook(&freepsbluetooth.HookBluetooth{})
	}
	telg := telegram.NewTelegramBot(cr, ge, cancel)
	mm.StartListening(ge)

	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		m.Shutdown()
		telg.Shutdown(context.TODO())
		http.Shutdown(context.TODO())
		mm.Shutdown()
	}
	running := ge.ReloadRequested()
	ge.Shutdown(base.NewContext(logger))
	logger.Infof("Stopping Listeners")
	return running
}

func main() {
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&operator, "m", "", "Specify operator to execute directly without starting listeners")
	flag.StringVar(&fn, "f", "", "Specify the operator function to call directly")
	flag.StringVar(&argstring, "a", "", "Specify arguments to function as urlencoded string")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.StringVar(&input, "i", "", "input file, use \"-\" to read from stdin")

	flag.Parse()

	for mainLoop() {
	}
}
