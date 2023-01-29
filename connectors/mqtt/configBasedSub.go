// methods here will be deprecated soon

package mqtt

import (
	"strings"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

func (fm *FreepsMqttImpl) processMessage(tc TopicConfig, message []byte, topic string) {
	input := freepsgraph.MakeByteOutput(message)
	graphName := tc.GraphToCall
	if graphName == "" {
		graphName = tc.TemplateToCall
	}

	// rather complicated logic that was introduced to push to Influx
	if len(tc.Fields) > 0 {
		value := string(message)
		t := strings.Split(topic, "/")
		field := ""
		if len(t) > tc.FieldIndex {
			field = t[tc.FieldIndex]
		}
		measurement := ""
		if len(t) > tc.MeasurementIndex {
			measurement = t[tc.MeasurementIndex]
		}
		fconf, fieldExists := tc.Fields[field]
		if fieldExists {
			fieldAlias := field
			if fconf.Alias != nil {
				fieldAlias = *fconf.Alias
			}
			if fconf.TrueValue != nil {
				if value == *fconf.TrueValue {
					value = "true"
				} else {
					value = "false"
				}
			}

			fwt := FieldWithType{fconf.Datatype, value}
			args := JsonArgs{Measurement: measurement, FieldsWithType: map[string]FieldWithType{fieldAlias: fwt}}

			input = freepsgraph.MakeObjectOutput(args)
		} else {
			fm.mqttlogger.WithFields(log.Fields{"topic": topic, "measurement": measurement, "field": field, "value": value}).Info("No field config found")
		}
	}
	ctx := utils.NewContext(fm.mqttlogger)
	out := fm.ge.ExecuteGraph(ctx, graphName, map[string]string{"topic": topic}, input)
	fm.publishResult(topic, ctx, out)
}
