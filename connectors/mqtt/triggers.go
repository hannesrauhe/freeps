package mqtt

import (
	"fmt"
	"net/http"
	"strings"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/utils"
)

func (fm *FreepsMqttImpl) executeTrigger(ctx *base.Context, topic string, message MQTT.Message) *base.OperatorIO {
	tags := []string{"mqtt", "topic:" + topic}
	input := base.MakeByteOutput(message.Payload())
	args := base.NewFunctionArguments(map[string]string{"topic": message.Topic(), "subscription": tags[1]})
	freepsstore.GetGlobalStore().GetNamespaceNoError("_mqtt").SetValue(message.Topic(), input, ctx)
	tParts := strings.Split(message.Topic(), "/")
	for ti, tp := range tParts {
		args.Append(fmt.Sprintf("topic%d", ti), tp)
	}
	out := fm.ge.ExecuteGraphByTags(ctx, tags, args, input)
	fm.publishResult(topic, ctx, out)
	return out
}

// GraphID auggestions returns suggestions for graph names
func (o *OpMQTT) GraphIDSuggestions() map[string]string {
	graphNames := map[string]string{}
	res := o.GE.GetAllGraphDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, o.GE)
		_, exists := graphNames[info.DisplayName]
		if !exists {
			graphNames[info.DisplayName] = id
		} else {
			graphNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return graphNames
}

type TopicTrigger struct {
	GraphID string
	Topic   string
}

// TopicSuggestions returns known topics
func (tt *TopicTrigger) TopicSuggestions(o *OpMQTT) []string {
	maxSize := 20
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_mqtt")
	if err != nil {
		return []string{}
	}
	allKeys := ns.GetKeys()
	if len(allKeys) <= maxSize {
		return allKeys
	}
	filteredKeys := allKeys
	if tt.Topic != "" {
		filteredKeys = []string{}
		for _, k := range allKeys {
			if utils.StringStartsWith(k, tt.Topic) {
				filteredKeys = append(filteredKeys, k)
				if len(filteredKeys) >= maxSize {
					break
				}
			}
		}
	}
	if len(filteredKeys) <= maxSize {
		return filteredKeys
	}

	h1Keys := []string{}
	lastPrefix := ""
	for _, k := range filteredKeys {
		if lastPrefix != "" && utils.StringStartsWith(k, lastPrefix) {
			continue
		}
		parts := strings.SplitN(k, "/", 2)
		lastPrefix = parts[0]
		h1Keys = append(h1Keys, lastPrefix)
		if len(h1Keys) >= maxSize {
			break
		}
	}
	return h1Keys
}

func (o *OpMQTT) SetTopicTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TopicTrigger) *base.OperatorIO {
	gd, found := o.GE.GetGraphDesc(args.GraphID)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", args.GraphID)
	}
	gd.AddTags("mqtt", "topic:"+args.Topic)
	err := o.GE.AddGraph(ctx, args.GraphID, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}
