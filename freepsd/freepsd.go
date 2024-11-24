package main

import (
	"bufio"
	"flag"
	"log"
	"os"

	logrus "github.com/sirupsen/logrus"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/hannesrauhe/freeps/base"
	opalert "github.com/hannesrauhe/freeps/connectors/alert"
	freepsbluetooth "github.com/hannesrauhe/freeps/connectors/bluetooth"
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
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool
var configpath, fn, operator, argstring, input string

type loggingConfig struct {
	Level            logrus.Level
	DisableTimestamp bool
	DisableQuote     bool
	Filename         string // Log file path
	MaxSize          int    // Max size in MB before rotating
	MaxBackups       int    // Max number of backup files
	MaxAge           int    // Max age in days before deleting
	Compress         bool   // Compress rotated files
}

func configureLogging(cr *utils.ConfigReader, logger *logrus.Logger) {
	level := logrus.InfoLevel
	if verbose {
		level = logrus.DebugLevel
	}
	loggingConfig := loggingConfig{
		Level:            level,
		DisableTimestamp: false,
		DisableQuote:     false,
		Filename:         "./log/freepsd/freepsd.log",
		MaxSize:          10,
		MaxBackups:       5,
		MaxAge:           30,
		Compress:         true,
	}
	cr.ReadSectionWithDefaults("logging", &loggingConfig)
	/*
		logger.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: loggingConfig.DisableTimestamp,
			DisableQuote:     loggingConfig.DisableQuote,
		})
	*/
	logger.SetFormatter(&logrus.JSONFormatter{})

	if !verbose {
		level = loggingConfig.Level
	}
	logger.SetLevel(level)

	logger.Infof("Logging to %v", loggingConfig.Filename)

	lumberjackLogger := &lumberjack.Logger{
		Filename:   loggingConfig.Filename,
		MaxSize:    loggingConfig.MaxSize,
		MaxBackups: loggingConfig.MaxBackups,
		MaxAge:     loggingConfig.MaxAge,
		Compress:   loggingConfig.Compress,
	}

	logger.SetOutput(lumberjackLogger)
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

	logger.Debug("Loading graph engine")

	ge := freepsgraph.NewGraphEngine(baseCtx, cr, cancel)

	// keep this here so the operators are re-created on reload
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge},
		&freepsbluetooth.Bluetooth{GE: ge},
		&muteme.MuteMe{GE: ge},
		&freepsflux.OperatorFlux{},
		&freepsutils.OpUtils{},
		&freepsutils.OpRegexp{},
		&freepsutils.OpGraphBuilder{GE: ge},
		&freepshttp.OpCurl{CR: cr, GE: ge},
		&telegram.OpTelegram{GE: ge},
		&pixeldisplay.OpPixelDisplay{},
		&opconfig.OpConfig{CR: cr, GE: ge},
		&optime.OpTime{},
		&fritz.OpFritz{CR: cr, GE: ge},
		&mqtt.OpMQTT{CR: cr, GE: ge},
		&opalert.OpAlert{CR: cr, GE: ge},
		&weather.OpWeather{},
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
			log.Fatal(err)
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
				log.Fatal(err)
			}
			oio = base.MakeByteOutput(content)
		}
		output := ge.ExecuteOperatorByName(baseCtx, operator, fn, fa, oio)
		if output != nil {
			output.WriteTo(os.Stdout, 1000)
		} else {
			logger.Error("Output of operator was nil")
		}
		ge.Shutdown(base.NewContext(logger, "Shutdown Context"))
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
		ge.Shutdown(base.NewContext(logger, "Shutdown Context"))
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
