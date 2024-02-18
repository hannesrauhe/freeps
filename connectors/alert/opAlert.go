package opalert

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// OpAlert is a FreepsOperator that can be used to retrieve and modify the config
type OpAlert struct {
	CR *utils.ConfigReader
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperator = &OpAlert{}

type Alert struct {
	Name      string
	Category  *string    `json:",omitempty"`
	Desc      *string    `json:",omitempty"`
	Severity  *int       `json:",omitempty"`
	ExpiresAt *time.Time `json:",omitempty"`
}

type AlertWithMetadata struct {
	Alert
	Counter int
	First   time.Time
	Last    time.Time // refactor UpdateTransaction to get StoreEntry which contains this info
}

func (a *AlertWithMetadata) IsExpired() bool {
	return a.ExpiresAt != nil && a.ExpiresAt.After(time.Now())
}

// SetAlert creates an stores a new alert
func (oc *OpAlert) SetAlert(ctx *base.Context, mainInput *base.OperatorIO, args Alert) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	var a AlertWithMetadata
	ns.UpdateTransaction(args.Name, func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			a = AlertWithMetadata{First: time.Now()}
		}
		if a.IsExpired() {
			a.Counter = 0
		}
		a.Counter++
		a.Last = time.Now()

		a.Alert = args
		return base.MakeObjectOutput(a)
	}, ctx.GetID())
	return base.MakeObjectOutput(a)
}

// SetAlert creates an stores a new alert
func (oc *OpAlert) ResetAlert(ctx *base.Context, mainInput *base.OperatorIO, args Alert) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	var a AlertWithMetadata
	ns.UpdateTransaction(args.Name, func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			a = AlertWithMetadata{First: time.Now()}
		}
		if !a.IsExpired() {
			eTime := time.Now()
			a.ExpiresAt = &eTime
		}

		a.Alert = args
		return base.MakeObjectOutput(a)
	}, ctx.GetID())
	return base.MakeObjectOutput(a)
}

// SetAlert creates an stores a new alert
func (oc *OpAlert) GetAlert(ctx *base.Context, mainInput *base.OperatorIO, args Alert) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	entry := ns.GetValue(args.Name)
	if entry == freepsstore.NotFoundEntry {
		return entry.GetData()
	}
	var a AlertWithMetadata
	entry.GetData().ParseJSON(&a)
	if a.IsExpired() {
		return base.MakeOutputError(http.StatusNotFound, "Key not found")
	}
	return entry.GetData()
}

func (o *OpAlert) GetHook() interface{} {
	return &HookAlert{o}
}
