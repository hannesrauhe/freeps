// methods here will be deprecated soon

package mqtt

import (
	"strings"

	"github.com/hannesrauhe/freeps/base"
	log "github.com/sirupsen/logrus"
)

type FieldConfig struct {
	Datatype  string
	Alias     *string // name used in influx
	TrueValue *string // if datatype==bool, this is used as true value, everything else is false
}

var DefaultTopicConfig = TopicConfig{Topic: "#", Qos: 0, MeasurementIndex: -1, FieldIndex: -1, Fields: map[string]FieldConfig{}, TemplateToCall: "mqttaction"}

type FieldWithType struct {
	FieldType  string
	FieldValue string
}
type JsonArgs struct {
	Measurement    string
	FieldsWithType map[string]FieldWithType
}

type TopicConfig struct {
	Topic string // Topic to subscribe to
	Qos   int    // The QoS to subscribe to messages at
	// the topic string is split by slash; the values of the resulting array can be used as measurement and field - the index can be specified here
	MeasurementIndex int                    // index that points to the measurement in the topic-array
	FieldIndex       int                    // index that points to the field in the topic-array
	Fields           map[string]FieldConfig `json:",omitempty"`
	TemplateToCall   string                 `json:",omitempty"`
	FlowToCall       string
}

func (fm *FreepsMqttImpl) processMessage(tc TopicConfig, message []byte, topic string) {
	input := base.MakeByteOutput(message)
	flowName := tc.FlowToCall
	if flowName == "" {
		flowName = tc.TemplateToCall
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

			input = base.MakeObjectOutput(args)
		} else {
			fm.ctx.GetLogger().WithFields(log.Fields{"topic": topic, "measurement": measurement, "field": field, "value": value}).Info("No field config found")
		}
	}
	ctx := base.CreateContextWithField(fm.ctx, "component", "mqtt", "MQTT topic: "+topic)
	out := fm.ge.ExecuteFlow(ctx, flowName, base.NewSingleFunctionArgument("topic", topic), input)
	fm.publishResult(topic, ctx, out)
}
