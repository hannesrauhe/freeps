package opalert

import (
	"fmt"
	"math"
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
	CR                *utils.ConfigReader
	GE                *freepsgraph.GraphEngine
	config            AlertConfig
	severityOverrides utils.CIMap[int]
}

var _ base.FreepsOperator = &OpAlert{}
var _ base.FreepsOperatorWithConfig = &OpAlert{}

func (oc *OpAlert) GetDefaultConfig() interface{} {
	cfg := AlertConfig{Enabled: true, SeverityOverrides: map[string]int{}}
	return &cfg
}

func (oc *OpAlert) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	opc := config.(*AlertConfig)
	return &OpAlert{CR: oc.CR, GE: oc.GE, config: *opc, severityOverrides: utils.NewCIMap(opc.SeverityOverrides)}, nil
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

func (oc *OpAlert) nameSuggestions(category *string, returnFullName bool) map[string]string {
	ret := map[string]string{}
	truePtr := true
	alerts, err := oc.getAlerts(GetAlertArgs{Category: category, IncludeExpired: &truePtr, IncludeSilenced: &truePtr})
	if err != nil {
		return ret
	}
	for _, k := range alerts {
		if returnFullName {
			ret[k.GetFullName()] = k.GetFullName()
		} else {
			ret[k.Name] = k.Name
		}
	}
	return ret
}

// updateAlert is a helper function to create a new or update an existing alert
func (oc *OpAlert) updateAlert(ctx *base.Context, category string, name string, updateFunc func(*AlertWithMetadata, bool)) (AlertWithMetadata, error) {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return AlertWithMetadata{}, fmt.Errorf("Error getting store: %v", err)
	}
	alertName := getAlertName(name, category)
	var a AlertWithMetadata
	io := ns.UpdateTransaction(alertName, func(oi base.OperatorIO) *base.OperatorIO {
		err := oi.ParseJSON(&a)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "Error parsing alert: %v", err)
		}

		newAlert := false
		if oi.IsEmpty() {
			a = AlertWithMetadata{
				Alert:   Alert{Name: name, Category: category, Severity: 5},
				Counter: 0,
				First:   time.Time{},
				Last:    time.Time{},
			}
			newAlert = true
		}
		updateFunc(&a, newAlert)
		return base.MakeObjectOutput(a)
	}, ctx)
	if io.IsError() {
		return AlertWithMetadata{}, io.GetError()
	}
	return a, nil
}

// SetAlert creates and stores a new alert
func (oc *OpAlert) SetAlert(ctx *base.Context, mainInput *base.OperatorIO, args Alert, addArgs base.FunctionArguments) *base.OperatorIO {
	execTrigger := false
	if oc.severityOverrides != nil {
		args.Severity = oc.severityOverrides.GetOrDefault(args.GetFullName(), args.Severity)
	}

	a, err := oc.updateAlert(ctx, args.Category, args.Name, func(a *AlertWithMetadata, newAlert bool) {
		if newAlert {
			a.First = time.Now()
			execTrigger = true
		}
		if a.IsExpired() {
			a.First = time.Now()
			execTrigger = true
		}
		a.Counter++
		a.Last = time.Now()

		a.Alert = args
	})

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error setting alert: %v", err)
	}

	if execTrigger && !addArgs.Has("noTrigger") {
		oc.execTriggers(ctx, a)
	}
	return base.MakeEmptyOutput()
}

type SilenceAlertArgs struct {
	Name            string
	Category        string
	SilenceDuration time.Duration
}

func (sa *SilenceAlertArgs) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(&sa.Category, false)
}

// SilenceAlert keeps the alert from triggering for the given duration
func (oc *OpAlert) SilenceAlert(ctx *base.Context, mainInput *base.OperatorIO, args SilenceAlertArgs) *base.OperatorIO {
	_, err := oc.updateAlert(ctx, args.Category, args.Name, func(a *AlertWithMetadata, newAlert bool) {
		a.SilenceUntil = time.Now().Add(args.SilenceDuration)
	})

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error setting alert: %v", err)
	}

	return base.MakeEmptyOutput()
}

type ResetAlertArgs struct {
	Name     string
	Category string
}

func (ra *ResetAlertArgs) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(&ra.Category, false)
}

// ResetAlert deletes the alert and resets the counter
func (oc *OpAlert) ResetAlert(ctx *base.Context, mainInput *base.OperatorIO, args ResetAlertArgs) *base.OperatorIO {
	execTriggers := false
	a, err := oc.updateAlert(ctx, args.Category, args.Name, func(a *AlertWithMetadata, newAlert bool) {
		if newAlert {
			// after restarts, the alert might be reset, so we need to execute the triggers
			execTriggers = true
		}

		if a.ExpiresInDuration == nil || !a.IsExpired() {
			eTime := time.Now().Sub(a.Last)
			a.ExpiresInDuration = &eTime
			execTriggers = true
		}

	})

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error resetting alert: %v", err)
	}

	if execTriggers {
		oc.execTriggers(ctx, a)
	}
	return base.MakeEmptyOutput()
}

// ResetSilence stops ignoring alerts
func (oc *OpAlert) ResetSilence(ctx *base.Context, mainInput *base.OperatorIO, args ResetAlertArgs) *base.OperatorIO {
	_, err := oc.updateAlert(ctx, args.Category, args.Name, func(a *AlertWithMetadata, newAlert bool) {
		a.SilenceUntil = time.Now()
	})

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error setting alert: %v", err)
	}

	return base.MakeEmptyOutput()
}

type GetAlertArgs struct {
	Severity        *int
	Category        *string
	IncludeSilenced *bool
	IncludeExpired  *bool
	UseTimestamps   *bool
}

// getAlerts returns all alerts matching the given args
func (oc *OpAlert) getAlerts(args GetAlertArgs) (map[string]ReadableAlert, error) {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	alerts := make(map[string]ReadableAlert, 0)
	if err != nil {
		return alerts, fmt.Errorf("Error getting store: %v", err)
	}
	for _, entry := range ns.GetSearchResultWithMetadata("", "", "", 0, math.MaxInt64) {
		var a AlertWithMetadata
		err := entry.ParseJSON(&a)
		if err != nil {
			continue // skip invalid alerts
		}
		if a.IsExpired() && (args.IncludeExpired == nil || *args.IncludeExpired == false) {
			continue
		}
		if args.Severity != nil && a.Severity > *args.Severity {
			continue
		}
		if args.Category != nil && a.Category != *args.Category {
			continue
		}
		if a.IsSilenced() && (args.IncludeSilenced == nil || *args.IncludeSilenced == false) {
			continue
		}

		alerts[a.GetFullName()] = NewReadableAlert(a, entry.GetModifiedBy())
	}
	return alerts, nil
}

// GetAlerts returns all alerts matching the given args and returns a map of alerts with readable timestamps and the modified-by-uuid
func (oc *OpAlert) GetAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	alerts, err := oc.getAlerts(args)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error getting store: %v", err)
	}

	return base.MakeObjectOutput(alerts)
}

// GetActiveAlerts is an alias for GetAlerts
func (oc *OpAlert) GetActiveAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	return oc.GetAlerts(ctx, mainInput, args)
}

// GetShortAlertString returns a single string describing all active alerts of a given severity
func (oc *OpAlert) GetShortAlertString(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getAlerts(args)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	alertNames := make([]string, 0)
	categories := make(map[string]int, 0)
	var a ReadableAlert // used if there is only one alert
	for _, a = range activeAlerts {
		categories[a.Category] = 1
		alertNames = append(alertNames, a.Name)
	}
	if len(activeAlerts) == 0 {
		return base.MakeEmptyOutput()
	}
	if len(activeAlerts) == 1 {
		if a.Desc != "" {
			return base.MakePlainOutput(a.Desc)
		}
		return base.MakeSprintfOutput("Active alert: %v", a.GetFullName())
	}

	alertListStr := ""
	if len(alertNames) <= 3 {
		alertListStr = strings.Join(alertNames, ",")
	}
	if len(categories) == 0 {
		return base.MakeSprintfOutput("%d alerts: %v", len(activeAlerts), alertListStr)
	}
	if len(categories) == 1 {
		for c := range categories {
			return base.MakeSprintfOutput("%d %v alerts: %v", len(activeAlerts), c, alertListStr)
		}
	}
	return base.MakeSprintfOutput("%d alerts: %v", len(activeAlerts), alertListStr)
}

// HasAlerts returns an empty output if there are any active alerts matching the criteria
func (oc *OpAlert) HasAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getAlerts(args)
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

// IsActiveAlertArgs is used to check if an alert is active
type IsActiveAlertArgs struct {
	Name           string
	Category       string
	IgnoreSilence  *bool
	ActiveDuration *time.Duration
}

func (iaa *IsActiveAlertArgs) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(&iaa.Category, false)
}

// GetActiveAlert returns alert information if the alert is active
func (oc *OpAlert) GetActiveAlert(ctx *base.Context, mainInput *base.OperatorIO, args IsActiveAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	var a AlertWithMetadata
	oi := ns.GetValue(getAlertName(args.Name, args.Category))
	if oi == freepsstore.NotFoundEntry {
		return base.MakeOutputError(http.StatusNotFound, "Alert %v does not exist", getAlertName(args.Name, args.Category))
	}
	err = oi.ParseJSON(&a)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error parsing alert: %v", err)
	}
	if a.IsExpired() {
		return base.MakeOutputError(http.StatusExpectationFailed, "Alert %v has expired", a.GetFullName())
	}
	if a.IsSilenced() && (args.IgnoreSilence == nil || *args.IgnoreSilence == false) {
		return base.MakeOutputError(http.StatusExpectationFailed, "Alert %v is silenced", a.GetFullName())
	}
	if args.ActiveDuration != nil && time.Now().Sub(a.First) < *args.ActiveDuration {
		return base.MakeOutputError(http.StatusExpectationFailed, "Alert %v has not been active for %v", a.GetFullName(), *args.ActiveDuration)
	}

	return base.MakeObjectOutput(NewReadableAlert(a, oi.GetModifiedBy()))
}

// IsActiveAlert returns an empty output if the alert is active
func (oc *OpAlert) IsActiveAlert(ctx *base.Context, mainInput *base.OperatorIO, args IsActiveAlertArgs) *base.OperatorIO {
	r := oc.GetActiveAlert(ctx, mainInput, args)
	if r.IsError() {
		return r
	}
	return base.MakeEmptyOutput()
}
