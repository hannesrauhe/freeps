//go:build !noinflux

package influx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

// OperatorInflux is that enabled InfluxDB Flux queries to be executed
type OperatorInflux struct {
	CR             *utils.ConfigReader
	GE             *freepsflow.FlowEngine
	config         *InfluxConfig
	client         influxdb2.Client
	writeApi       api.WriteAPI
	storeNamespace freepsstore.StoreNamespace
}

var _ base.FreepsOperatorWithConfig = &OperatorInflux{}
var _ base.FreepsOperatorWithShutdown = &OperatorInflux{}

// GetDefaultConfig returns a copy of the default config
func (o *OperatorInflux) GetDefaultConfig() interface{} {
	return &InfluxConfig{Enabled: true, WriteAlertSeverity: 3, WriteAlertDuration: time.Minute}
}

func (o *OperatorInflux) migrateConfig(cfg *InfluxConfig) {
	oldCfgSection := &OldFreepsFluxConfig{}
	oldSectionName := "freepsflux"
	o.CR.ReadSectionWithDefaults(oldSectionName, oldCfgSection)
	if len(oldCfgSection.InfluxdbConnections) == 0 {
		oldSectionName = "flux"
		o.CR.ReadSectionWithDefaults(oldSectionName, oldCfgSection)
		if len(oldCfgSection.InfluxdbConnections) == 0 {
			return
		}
	}
	oldCfg := oldCfgSection.InfluxdbConnections[0]
	cfg.Bucket = oldCfg.Bucket
	cfg.Org = oldCfg.Org
	cfg.Token = oldCfg.Token
	cfg.URL = oldCfg.URL
	cfg.Enabled = oldCfgSection.Enabled
	if len(oldCfgSection.InfluxdbConnections) > 1 {
		oldCfgSection.InfluxdbConnections = oldCfgSection.InfluxdbConnections[1:]
		o.CR.WriteSection(oldSectionName, oldCfgSection, true)
	} else {
		oldCfgSection.InfluxdbConnections = nil
		o.CR.RemoveSection(oldSectionName)
	}

}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (o *OperatorInflux) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*InfluxConfig)
	if cfg.URL == "" {
		o.migrateConfig(cfg)

		if cfg.Enabled == false {
			return nil, fmt.Errorf("Old freepsflux config found, but disabled, disabling InfluxDB operator")
		}
	}

	if (cfg.URL == "" || cfg.Token == "" || cfg.Bucket == "" || cfg.Org == "") && cfg.StoreNamespace == "" {
		return nil, errors.New("Failed to create InfluxDB client, settings are not complete")
	}

	if cfg.StoreNamespace != "" && cfg.URL != "" {
		return nil, errors.New("Both store namespace and InfluxDB URL are set, please use only one")
	}

	newOp := &OperatorInflux{
		CR:     o.CR,
		GE:     o.GE,
		config: cfg,
	}
	instanceName := "default"
	if len(name) > len("influx") {
		instanceName = name[len("influx."):]
	}
	if globalInflux == nil {
		globalInflux = make(map[string]*OperatorInflux)
	}
	globalInflux[instanceName] = newOp
	return newOp, nil
}

func (o *OperatorInflux) StartListening(ctx *base.Context) {
	cfg := o.config

	if cfg.StoreNamespace != "" {
		s := freepsstore.GetGlobalStore()
		if s == nil {
			o.GE.SetSystemAlert(ctx, "init_error", "influx", 2, errors.New("Store is not initialized"), &o.config.WriteAlertDuration)
			return
		}
		ns, err := s.CreateNamespace(cfg.StoreNamespace, freepsstore.StoreNamespaceConfig{
			NamespaceType: "log",
			AutoTrim:      100,
		})
		if err != nil {
			o.GE.SetSystemAlert(ctx, "init_error", "influx", 2, fmt.Errorf("Failed to get store namespace %v: %v", cfg.StoreNamespace, err), &o.config.WriteAlertDuration)
			return
		}
		o.storeNamespace = ns
	}

	influxOptions := influxdb2.DefaultOptions()
	client := influxdb2.NewClientWithOptions(cfg.URL, cfg.Token, influxOptions)
	if client == nil {
		o.GE.SetSystemAlert(ctx, "init_error", "influx", 2, errors.New("Failed to create InfluxDB client, check connection settings"), &o.config.WriteAlertDuration)
		return
	}
	o.client = client
	o.writeApi = client.WriteAPI(cfg.Org, cfg.Bucket)
	if o.writeApi == nil {
		o.GE.SetSystemAlert(ctx, "init_error", "influx", 2, errors.New("Failed to create InfluxDB write API, check connection settings"), &o.config.WriteAlertDuration)
		return
	}

	errorsCh := o.writeApi.Errors()
	go func() {
		for err := range errorsCh {
			o.GE.SetSystemAlert(ctx, "write_error", "influx", o.config.WriteAlertSeverity, err, &o.config.WriteAlertDuration)
		}
	}()
}

func (o *OperatorInflux) Shutdown(ctx *base.Context) {
	if o.writeApi != nil {
		o.writeApi.Flush()
	}
	if o.client != nil {
		o.client.Close()
	}
	// Set the client and writeApi to nil to avoid memory leaks
	o.client = nil
	o.writeApi = nil
}

type FieldWithType struct {
	FieldType  string
	FieldValue string
}
type JsonArgs struct {
	Measurement      string
	Tags             map[string]string
	Fields           map[string]interface{}
	FieldsWithType   map[string]FieldWithType
	DefaultFieldType string
}

func changeFieldType(fieldValue interface{}, fieldType string) (interface{}, error) {
	var value interface{}
	var err error
	fieldType = strings.ToLower(fieldType)
	switch fieldType {
	case "float", "float64":
		value, err = utils.ConvertToFloat(fieldValue)
	case "int", "int64":
		value, err = utils.ConvertToInt64(fieldValue)
	case "bool":
		value, err = utils.ConvertToBool(fieldValue)
	default:
		value = fieldValue
	}
	if err != nil {
		return value, fmt.Errorf("Error when converting: \"%v\" does not seem to be of type \"%v\": %v", fieldValue, fieldType, err)
	}
	return value, nil
}

func (o *OperatorInflux) PushFields(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	fields := map[string]interface{}{}

	var args JsonArgs
	var err error
	input.ParseJSON(&args)
	if len(args.Measurement) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "Name of measurement is empty")
	}
	for k, v := range args.Fields {
		fields[k], err = changeFieldType(v, args.DefaultFieldType)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
		}
	}
	for k, v := range args.FieldsWithType {
		fields[k], err = changeFieldType(v.FieldValue, v.FieldType)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
		}
	}

	return o.PushFieldsInternal(args.Measurement, args.Tags, fields, ctx)
}

type PushArguments struct {
	Measurement string
	Field       string
	FieldType   *string
}

func (o *OperatorInflux) FieldTypeSuggestions() []string {
	return []string{"float", "float64", "int", "int64", "bool"}
}

func (o *OperatorInflux) PushSingleField(ctx *base.Context, input *base.OperatorIO, args PushArguments, tags base.FunctionArguments) *base.OperatorIO {
	fields := map[string]interface{}{}
	var err error
	if args.FieldType == nil {
		fields[args.Field] = input.Output
	} else {
		fields[args.Field], err = changeFieldType(input.Output, *args.FieldType)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err)
	}

	return o.PushFieldsInternal(args.Measurement, tags.GetOriginalCaseMapOnlyFirst(), fields, ctx)
}

type PushMeasurementArguments struct {
	Measurement string
}

func (o *OperatorInflux) PushMeasurement(ctx *base.Context, input *base.OperatorIO, args PushMeasurementArguments, tags base.FunctionArguments) *base.OperatorIO {
	if input.IsEmpty() {
		return base.MakeOutputError(http.StatusBadRequest, "no input")
	}
	fields := map[string]interface{}{}
	err := input.ParseJSON(&fields)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Could not parse input: %v", err)
	}
	if len(fields) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "empty fields map")
	}

	return o.PushFieldsInternal(args.Measurement, tags.GetOriginalCaseMapOnlyFirst(), fields, ctx)
}
