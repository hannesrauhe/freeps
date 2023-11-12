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
	opconfig "github.com/hannesrauhe/freeps/connectors/config"
	freepsexec "github.com/hannesrauhe/freeps/connectors/exec"
	"github.com/hannesrauhe/freeps/connectors/freepsflux"
	"github.com/hannesrauhe/freeps/connectors/fritz"
	freepshttp "github.com/hannesrauhe/freeps/connectors/http"
	"github.com/hannesrauhe/freeps/connectors/mqtt"
	"github.com/hannesrauhe/freeps/connectors/muteme"
	"github.com/hannesrauhe/freeps/connectors/pixeldisplay"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/connectors/telegram"
	optime "github.com/hannesrauhe/freeps/connectors/time"
	"github.com/hannesrauhe/freeps/connectors/ui"
	freepsutils "github.com/hannesrauhe/freeps/connectors/utils"
	"github.com/hannesrauhe/freeps/connectors/weather"
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

	// keep this here so the operators are re-created on reload
	availableOperators := []base.FreepsOperator{
		&freepsbluetooth.Bluetooth{GE: ge},
		&muteme.MuteMe{GE: ge},
		&freepsflux.OperatorFlux{},
		&freepsutils.OpUtils{},
		&freepsutils.OpRegexp{},
		&freepsutils.OpGraphBuilder{GE: ge},
		&freepshttp.OpCurl{CR: cr, GE: ge},
		&chaosimradio.OpCiR{},
		&telegram.OpTelegram{},
		&pixeldisplay.OpPixelDisplay{},
		&opconfig.OpConfig{CR: cr, GE: ge},
		&optime.OpTime{},
		&fritz.OpFritz{},
<<<<<<< HEAD
		&mqtt.OpMQTT{CR: cr, GE: ge},
=======
		&weather.OpWeather{},
>>>>>>> main
	}

	ge.AddOperator(freepsstore.NewOpStore(cr, ge)) //needs to be first for now
	for _, op := range availableOperators {
		// this will automatically skip operators that are not enabled in the config
		ge.AddOperators(base.MakeFreepsOperators(op, cr, initCtx))
	}
	ge.AddOperator(wled.NewWLEDOp(cr))
	ge.AddOperator(ui.NewHTMLUI(cr, ge))
	freepsexec.AddExecOperators(cr, ge)

	sh, err := freepsstore.NewStoreHook(cr, ge)
	if err != nil {
		logger.Errorf("Store hook not available: %v", err.Error())
	} else {
		ge.AddHook(sh)
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
	ge.StartListening(initCtx)

<<<<<<< HEAD
	fbt, err := freepsbluetooth.NewBTWatcher(logger, cr, ge)
	if err != nil {
		logger.Errorf("FreepsBT not started: %v", err)
	} else if fbt != nil {
		ge.AddHook(&freepsbluetooth.HookBluetooth{})
=======
	m := mqtt.GetInstance()
	if err := m.Init(logger, cr, ge); err != nil {
		logger.Errorf("MQTT not started: %v", err)
	} else {
		h, _ := mqtt.NewMQTTHook(cr)
		ge.AddHook(h)
>>>>>>> main
	}
	telg := telegram.NewTelegramBot(cr, ge, cancel)

	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		telg.Shutdown(context.TODO())
	}
	running := ge.ReloadRequested()
	logger.Infof("Stopping Listeners")
	ge.Shutdown(base.NewContext(logger))
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
