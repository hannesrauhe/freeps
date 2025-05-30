package main

import (
	"bufio"
	"flag"
	"os"

	logrus "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freeps/base"
	opalert "github.com/hannesrauhe/freeps/connectors/alert"
	freepsbluetooth "github.com/hannesrauhe/freeps/connectors/bluetooth"
	opconfig "github.com/hannesrauhe/freeps/connectors/config"
	freepsexec "github.com/hannesrauhe/freeps/connectors/exec"
	"github.com/hannesrauhe/freeps/connectors/flowbuilder"
	"github.com/hannesrauhe/freeps/connectors/freepsflux"
	"github.com/hannesrauhe/freeps/connectors/fritz"
	freepshttp "github.com/hannesrauhe/freeps/connectors/http"
	freepsmetrics "github.com/hannesrauhe/freeps/connectors/metrics"
	"github.com/hannesrauhe/freeps/connectors/mqtt"
	"github.com/hannesrauhe/freeps/connectors/muteme"
	"github.com/hannesrauhe/freeps/connectors/pixeldisplay"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/connectors/telegram"
	optime "github.com/hannesrauhe/freeps/connectors/time"
	"github.com/hannesrauhe/freeps/connectors/ui"
	freepsutils "github.com/hannesrauhe/freeps/connectors/utils"
	"github.com/hannesrauhe/freeps/connectors/weather"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool
var configpath, fn, operator, argstring, input string

type loggingConfig struct {
	Level            logrus.Level
	DisableTimestamp bool
	DisableQuote     bool
	JSONFormatter    bool
}

func configureLogging(cr *utils.ConfigReader, logger *logrus.Logger) {
	level := logrus.InfoLevel
	if verbose {
		level = logrus.DebugLevel
	}
	loggingConfig := loggingConfig{Level: level, DisableTimestamp: false, DisableQuote: false, JSONFormatter: false}
	cr.ReadSectionWithDefaults("logging", &loggingConfig)
	if loggingConfig.JSONFormatter {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: loggingConfig.DisableTimestamp,
			DisableQuote:     loggingConfig.DisableQuote,
		})
	}
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

	baseCtx, cancel := base.NewBaseContext(logger)
	defer cancel()

	_, err = utils.GetTempDir()
	if err != nil {
		logger.Fatal("Temp dir creation failed: ", err.Error())
	}
	defer utils.DeleteTempDir()

	logger.Debug("Loading flow engine")

	ge := freepsflow.NewFlowEngine(baseCtx, cr, cancel)

	// keep this here so the operators are re-created on reload
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge}, // must be first so that other operators can use the store
		&opalert.OpAlert{CR: cr, GE: ge},     // must be second so that other operators can use alerts
		&sensor.OpSensor{CR: cr, GE: ge},     // must be third so that other operators can use sensors
		&freepsbluetooth.Bluetooth{GE: ge},
		&muteme.MuteMe{GE: ge},
		&freepsflux.OperatorFlux{},
		&freepsutils.OpUtils{},
		&freepsutils.OpMath{},
		&freepsutils.OpRegexp{},
		&flowbuilder.OpFlowBuilder{GE: ge},
		&freepshttp.OpCurl{CR: cr, GE: ge},
		&telegram.OpTelegram{GE: ge},
		&pixeldisplay.OpPixelDisplay{},
		&opconfig.OpConfig{CR: cr, GE: ge},
		&optime.OpTime{},
		&fritz.OpFritz{CR: cr, GE: ge},
		&mqtt.OpMQTT{CR: cr, GE: ge},
		&weather.OpWeather{},
		&freepsmetrics.OpMetrics{CR: cr, GE: ge},
	}

	for _, op := range availableOperators {
		// this will automatically skip operators that are not enabled in the config
		ge.AddOperators(base.MakeFreepsOperators(op, cr, baseCtx))
	}
	ge.AddOperator(ui.NewHTMLUI(cr, ge))
	freepsexec.AddExecOperators(cr, ge)

	if operator != "" {
		fa, err := base.NewFunctionArgumentsFromURLQuery(argstring)
		if err != nil {
			logger.Fatal(err)
		}
		oio := base.MakeEmptyOutput()

		if input == "-" {
			scanner := bufio.NewScanner(os.Stdin)
			b := []byte{}
			for scanner.Scan() {
				b = append(b, scanner.Bytes()...)
			}
			oio = base.MakeByteOutput(b)
		} else if input != "" {
			content, err := os.ReadFile(input)
			if err != nil {
				logger.Fatal(err)
			}
			oio = base.MakeByteOutput(content)
		}
		output := ge.ExecuteOperatorByName(baseCtx, operator, fn, fa, oio)
		if output != nil {
			output.WriteTo(os.Stdout, 1000)
		} else {
			logger.Error("Output of operator was nil")
		}
		ge.Shutdown(base.NewBaseContextWithReason(logger, "Shutdown Context"))
		return false
	}

	logger.Infof("Starting Listeners")
	ge.StartListening(baseCtx)
	logger.Infof("Listeners successfully started")

	keepRunning := true
	select {
	case <-baseCtx.Done():
		keepRunning = ge.ReloadRequested()
		logger.Infof("Stopping Listeners")
		ge.Shutdown(base.NewBaseContextWithReason(logger, "Shutdown Context"))
	}
	logger.Infof("All listeners stopped")
	return keepRunning
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
