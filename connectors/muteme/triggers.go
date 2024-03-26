package muteme

import (
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
)

func (mm *MuteMe) setTrigger(ctx *base.Context, graphId string, tags ...string) *base.OperatorIO {
	gd, found := mm.GE.GetGraphDesc(graphId)
	if !found {
		return base.MakeOutputError(http.StatusInternalServerError, "Couldn't find graph: %v", graphId)
	}

	gd.AddTags(mm.config.Tag)
	gd.AddTags(tags...)
	err := mm.GE.AddGraph(ctx, graphId, *gd, true)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Cannot modify graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

type TouchTrigger struct {
	GraphID string
}

// GraphID auggestions returns suggestions for graph names
func (mm *MuteMe) GraphIDSuggestions() map[string]string {
	graphNames := map[string]string{}
	res := mm.GE.GetAllGraphDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, mm.GE)
		_, exists := graphNames[info.DisplayName]
		if !exists {
			graphNames[info.DisplayName] = id
		} else {
			graphNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return graphNames
}

// SetTouchTrigger
func (mm *MuteMe) SetTouchTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TouchTrigger) *base.OperatorIO {
	return mm.setTrigger(ctx, args.GraphID, mm.config.TouchTag)
}

// SetMultiTouchTrigger
func (mm *MuteMe) SetMultiTouchTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TouchTrigger) *base.OperatorIO {
	return mm.setTrigger(ctx, args.GraphID, mm.config.MultiTouchTag)
}

// SetLongTouchTrigger
func (mm *MuteMe) SetLongTouchTrigger(ctx *base.Context, mainInput *base.OperatorIO, args TouchTrigger) *base.OperatorIO {
	return mm.setTrigger(ctx, args.GraphID, mm.config.LongTouchTag)
}

/* TODO(HR): call this in mainloop
func (mm *MuteMe) execTriggers(causedByCtx *base.Context, ) *base.OperatorIO {
	tags := []string{mm.config.Tag}
	args := map[string]string{}
	if tpress2.Sub(tpress1) < mm.config.MultiTouchDuration {
		tags = append(tags, mm.config.MultiTouchTag)
		args["TouchCount"] = fmt.Sprint(lastTouchCounter)
	} else {
		if lastTouchDuration > mm.config.LongTouchDuration {
			tags = append(tags, mm.config.LongTouchTag)
		} else {
			tags = append(tags, mm.config.TouchTag)
		}
		args["TouchDuration"] = lastTouchDuration.String()
	}
	resultIO := m.GE.ExecuteGraphByTags(base.NewContext(m.logger), tags, args, base.MakeEmptyOutput())
}
*/
