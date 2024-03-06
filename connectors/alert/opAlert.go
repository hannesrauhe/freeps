package opalert

import (
	"fmt"
	"net/http"
	"strings"
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
	Name              string
	Category          *string `json:",omitempty"`
	Desc              *string `json:",omitempty"`
	Severity          int
	ExpiresInDuration *time.Duration `json:",omitempty"`
}

type AlertWithMetadata struct {
	Alert
	Counter int
	First   time.Time
	Last    time.Time // refactor UpdateTransaction to get StoreEntry which contains this info
}

func (a *AlertWithMetadata) IsExpired() bool {
	if a.ExpiresInDuration == nil {
		return false
	}
	expiresAt := a.Last.Add(*a.ExpiresInDuration)
	return expiresAt.Before(time.Now())
}

func (a *Alert) GetFullName() string {
	cat := ""
	if a.Category != nil {
		cat = *a.Category
	}
	return fmt.Sprintf("%v.%v", cat, a.Name)
}

func (oc *OpAlert) SeveritySuggestions() []string {
	return []string{"1", "2", "3", "4", "5"}
}

func (oc *OpAlert) CategorySuggestions() []string {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return []string{}
	}
	cat := map[string]int{}
	for _, k := range ns.GetKeys() {
		c, _, found := strings.Cut(k, ".")
		if found {
			cat[c] = 1
		}
	}
	ret := []string{}
	for c := range cat {
		ret = append(ret, c)
	}
	return ret
}

func (oc *OpAlert) NameSuggestions() []string {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return []string{}
	}
	ret := []string{}
	for _, k := range ns.GetKeys() {
		c, n, found := strings.Cut(k, ".")
		if found {
			ret = append(ret, n)
		} else {
			ret = append(ret, c)
		}
	}
	return ret
}

// SetAlert creates and stores a new alert
func (oc *OpAlert) SetAlert(ctx *base.Context, mainInput *base.OperatorIO, args Alert) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	var a AlertWithMetadata
	ns.UpdateTransaction(args.GetFullName(), func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			a = AlertWithMetadata{First: time.Now()}
		}
		if a.IsExpired() {
			a.First = time.Now()
		}
		a.Counter++
		a.Last = time.Now()

		a.Alert = args
		return base.MakeObjectOutput(a)
	}, ctx.GetID())
	return base.MakeObjectOutput(a)
}

type ResetAlertArgs struct {
	Name     string
	Category *string
}

// ResetAlert resets an alerts
func (oc *OpAlert) ResetAlert(ctx *base.Context, mainInput *base.OperatorIO, args ResetAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	tempAlert := Alert{Name: args.Name, Category: args.Category, Severity: 5} // just to get the name
	var a AlertWithMetadata
	ns.UpdateTransaction(tempAlert.GetFullName(), func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			a = AlertWithMetadata{
				Alert:   tempAlert,
				Counter: 0,
				First:   time.Time{},
				Last:    time.Time{},
			}
		}
		if a.ExpiresInDuration == nil || !a.IsExpired() {
			eTime := time.Now().Sub(a.Last)
			a.ExpiresInDuration = &eTime
		}

		return base.MakeObjectOutput(a)
	}, ctx.GetID())
	return base.MakeObjectOutput(a)
}

type GetAlertArgs struct {
	Severity *int
	Category *string
}

func (oc *OpAlert) getActiveAlerts(args GetAlertArgs) ([]AlertWithMetadata, error) {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return make([]AlertWithMetadata, 0), fmt.Errorf("Error getting store: %v", err)
	}
	activeAlerts := make([]AlertWithMetadata, 0)
	for _, entry := range ns.GetAllValues(1000) {
		var a AlertWithMetadata
		entry.ParseJSON(&a)
		if a.IsExpired() {
			continue
		}
		if args.Severity != nil && a.Severity > *args.Severity {
			continue
		}
		if args.Category != nil && (a.Category == nil || *a.Category != *args.Category) {
			continue
		}

		activeAlerts = append(activeAlerts, a)
	}
	return activeAlerts, nil
}

// GetActiveString returns a single string describing all active alerts of a given severity
func (oc *OpAlert) GetActiveAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getActiveAlerts(args)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	return base.MakeObjectOutput(activeAlerts)
}

// GetShortAlertString returns a single string describing all active alerts of a given severity
func (oc *OpAlert) GetShortAlertString(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getActiveAlerts(args)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	alertNames := make([]string, 0)
	categories := make(map[string]int, 0)
	for _, a := range activeAlerts {
		if a.Category != nil {
			categories[*a.Category] = 1
		}
		alertNames = append(alertNames, a.Name)
	}
	if len(activeAlerts) == 0 {
		return base.MakeEmptyOutput()
	}
	if len(activeAlerts) == 1 {
		a := activeAlerts[0]
		if a.Desc != nil {
			return base.MakeSprintfOutput("Alert: %v", *a.Desc)
		}
		if a.Category == nil {
			return base.MakeSprintfOutput("Alert: %v", a.Name)
		}
		return base.MakeSprintfOutput("Alert %v in category %v", a.Name, *a.Category)
	}

	alertListStr := ""
	if len(alertNames) <= 3 {
		alertListStr = ": " + strings.Join(alertNames, ",")
	}
	if len(categories) == 0 {
		return base.MakeSprintfOutput("%d alerts%v", len(activeAlerts), alertListStr)
	}
	if len(categories) == 1 {
		for c := range categories {
			return base.MakeSprintfOutput("%d alerts in category %v%v", len(activeAlerts), c, alertListStr)
		}
	}
	return base.MakeSprintfOutput("%d alerts in %d categories%v", len(activeAlerts), len(categories), alertListStr)
}

// HasAlerts returns an empty output if there are any active alerts matching the criteria
func (oc *OpAlert) HasAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getActiveAlerts(args)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	if len(activeAlerts) > 0 {
		return base.MakeEmptyOutput()
	}
	return base.MakeOutputError(http.StatusExpectationFailed, "no alerts")
}

func (o *OpAlert) GetHook() interface{} {
	return &HookAlert{o}
}
