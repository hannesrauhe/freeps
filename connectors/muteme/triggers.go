//go:build !nomuteme && linux

package muteme

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
)

func (mm *MuteMe) setTrigger(ctx *base.Context, flowId string, tags ...string) *base.OperatorIO {
	gd, found := mm.GE.GetFlowDesc(flowId)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find flow: %v", flowId)
	}

	gd.AddTags(mm.config.Tag)
	gd.AddTags(tags...)
	err := mm.GE.AddFlow(ctx, flowId, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify flow: %v", err)
	}

	return base.MakeEmptyOutput()
}

type TouchTrigger struct {
	FlowID string
}

// FlowIDSuggestions returns suggestions for flow names
func (mm *MuteMe) FlowIDSuggestions() map[string]string {
	flowNames := map[string]string{}
	res := mm.GE.GetAllFlowDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, mm.GE)
		_, exists := flowNames[info.DisplayName]
		if !exists {
			flowNames[info.DisplayName] = id
		} else {
			flowNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return flowNames
}

// SetTouchTrigger
func (mm *MuteMe) SetTouchTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TouchTrigger) *base.OperatorIO {
	return mm.setTrigger(ctx, args.FlowID, mm.config.TouchTag)
}

// SetMultiTouchTrigger
func (mm *MuteMe) SetMultiTouchTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TouchTrigger) *base.OperatorIO {
	return mm.setTrigger(ctx, args.FlowID, mm.config.MultiTouchTag)
}

// SetLongTouchTrigger
func (mm *MuteMe) SetLongTouchTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TouchTrigger) *base.OperatorIO {
	return mm.setTrigger(ctx, args.FlowID, mm.config.LongTouchTag)
}

func (mm *MuteMe) execTriggers(parentCtx *base.Context, touchDuration time.Duration, lastTouchDuration time.Duration, lastTouchCounter int) *base.OperatorIO {
	tags := []string{mm.config.Tag}
	args := base.MakeEmptyFunctionArguments()
	var ctx *base.Context
	if touchDuration < mm.config.MultiTouchDuration {
		tags = append(tags, mm.config.MultiTouchTag)
		args.Append("TouchCount", fmt.Sprint(lastTouchCounter))
		ctx = base.CreateContextWithField(parentCtx, "component", "MuteMe", "MuteMe TouchCount"+fmt.Sprint(lastTouchCounter))
	} else {
		if lastTouchDuration > mm.config.LongTouchDuration {
			tags = append(tags, mm.config.LongTouchTag)
			ctx = base.CreateContextWithField(parentCtx, "component", "MuteMe", "MuteMe LongTouch")
		} else {
			tags = append(tags, mm.config.TouchTag)
			ctx = base.CreateContextWithField(parentCtx, "component", "MuteMe", "MuteMe Touch")
		}
		args.Append("TouchDuration", lastTouchDuration.String())
	}
	gs := sensor.GetGlobalSensors()
	if gs != nil {
		gs.SetSensorPropertyInternal(ctx, mm.config.Tag, mm.config.Tag, "TouchDuration", lastTouchDuration)
		gs.SetSensorPropertyInternal(ctx, mm.config.Tag, mm.config.Tag, "TouchCount", lastTouchCounter)
	}

	return mm.GE.ExecuteFlowByTags(ctx, tags, args, base.MakeEmptyOutput())
}
