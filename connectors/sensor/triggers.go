package sensor

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

// FlowID suggestions returns suggestions for flow names
func (o *OpSensor) FlowIDSuggestions() map[string]string {
	flowNames := map[string]string{}
	res := o.GE.GetAllFlowDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, o.GE)
		_, exists := flowNames[info.DisplayName]
		if !exists {
			flowNames[info.DisplayName] = id
		} else {
			flowNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return flowNames
}

func (o *OpSensor) executeTrigger(ctx *base.Context, sensorCategory string, sensorName string, changedProperties []string) *base.OperatorIO {
	// TODO(HR): async?
	categorySelectTags := []string{fmt.Sprintf("sensorCategory:%v", sensorCategory), "sensorCategory:*"}
	nameSelectTags := []string{fmt.Sprintf("sensorName:%v", sensorName), "sensorName:*"}
	propertySelectTags := []string{"sensorProperty:*"}
	for _, property := range changedProperties {
		propertySelectTags = append(propertySelectTags, fmt.Sprintf("sensorProperty:%v", property))
	}
	tagGroups := [][]string{{"sensor"}, categorySelectTags, propertySelectTags, nameSelectTags}
	args := base.MakeEmptyFunctionArguments()
	return o.GE.ExecuteFlowByTagsExtended(ctx, tagGroups, args, base.MakeEmptyOutput())
}

func (o *OpSensor) setTrigger(ctx *base.Context, flowId string, tags ...string) *base.OperatorIO {
	gd, found := o.GE.GetFlowDesc(flowId)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find flow: %v", flowId)
	}

	gd.AddTags("sensor")
	gd.AddTags(tags...)
	err := o.GE.AddFlow(ctx, flowId, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify flow: %v", err)
	}

	return base.MakeEmptyOutput()
}

type SetTriggerArgs struct {
	FlowID          string
	SensorCategory  *string
	SensorName      *string
	ChangedProperty *string
}

func (o *OpSensor) SetSensorTrigger(ctx *base.Context, input *base.OperatorIO, args SetTriggerArgs) *base.OperatorIO {
	tags := []string{}
	if args.SensorCategory != nil {
		tags = append(tags, fmt.Sprintf("sensorCategory:%v", *args.SensorCategory))
	} else {
		tags = append(tags, "sensorCategory:*")
	}

	if args.SensorName != nil {
		tags = append(tags, fmt.Sprintf("sensorName:%v", *args.SensorName))
	} else {
		tags = append(tags, "sensorName:*")
	}

	if args.ChangedProperty != nil {
		tags = append(tags, fmt.Sprintf("sensorProperty:%v", *args.ChangedProperty))
	} else {
		tags = append(tags, "sensorProperty:*")
	}

	return o.setTrigger(ctx, args.FlowID, tags...)
}
