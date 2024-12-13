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

type Alert struct {
	Name              string
	Category          *string `json:",omitempty"`
	Desc              *string `json:",omitempty"`
	Severity          int
	ExpiresInDuration *time.Duration `json:",omitempty"`
}

type AlertWithMetadata struct {
	Alert
	Counter      int
	First        time.Time
	Last         time.Time
	SilenceUntil time.Time
}

func (a *AlertWithMetadata) IsExpired() bool {
	if a.ExpiresInDuration == nil {
		return false
	}
	expiresAt := a.Last.Add(*a.ExpiresInDuration)
	return expiresAt.Before(time.Now())
}

func (a *AlertWithMetadata) IsSilenced() bool {
	return a.SilenceUntil.After(time.Now())
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

func (oc *OpAlert) nameSuggestions(category *string, returnFullName bool) map[string]string {
	ret := map[string]string{}
	truePtr := true
	alerts, err := oc.getAlertList(GetAlertArgs{Category: category, IncludeExpired: &truePtr, IncludeSilenced: &truePtr})
	if err != nil {
		return ret
	}
	for _, k := range alerts {
		if returnFullName {
			ret[k.GetFullName()] = k.GetFullName()
		} else {
			ret[k.Name] = k.GetFullName()
		}
	}
	return ret
}

func (aa *Alert) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(aa.Category, false)
}

// SetAlert creates and stores a new alert
func (oc *OpAlert) SetAlert(ctx *base.Context, mainInput *base.OperatorIO, args Alert, addArgs base.FunctionArguments) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	execTrigger := false
	alertIdentifier := args.GetFullName()
	if oc.severityOverrides != nil { // just a fix for testing
		args.Severity = oc.severityOverrides.GetOrDefault(alertIdentifier, args.Severity)
	}
	var a AlertWithMetadata
	ns.UpdateTransaction(alertIdentifier, func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			a = AlertWithMetadata{First: time.Now()}
			execTrigger = true
		}
		if a.IsExpired() {
			a.First = time.Now()
			execTrigger = true
		}
		a.Counter++
		a.Last = time.Now()

		a.Alert = args
		return base.MakeObjectOutput(a)
	}, ctx)

	if execTrigger && !addArgs.Has("noTrigger") {
		oc.execTriggers(ctx, a)
	}
	return base.MakeEmptyOutput()
}

type SilenceAlertArgs struct {
	Name            string
	Category        *string
	SilenceDuration time.Duration
}

func (sa *SilenceAlertArgs) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(sa.Category, false)
}

// SilenceAlert keeps the alert from triggering for the given duration
func (oc *OpAlert) SilenceAlert(ctx *base.Context, mainInput *base.OperatorIO, args SilenceAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	tempAlert := Alert{Name: args.Name, Category: args.Category, Severity: 5} // just to get the name
	var a AlertWithMetadata
	ns.UpdateTransaction(tempAlert.GetFullName(), func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			return base.MakeOutputError(http.StatusNotFound, "Alert %v does not exist", tempAlert.GetFullName())
		}
		a.SilenceUntil = time.Now().Add(args.SilenceDuration)

		return base.MakeObjectOutput(a)
	}, ctx)
	return base.MakeEmptyOutput()
}

type ResetAlertArgs struct {
	Name     string
	Category *string
}

func (ra *ResetAlertArgs) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(ra.Category, false)
}

// ResetAlert deletes the alert and resets the counter
func (oc *OpAlert) ResetAlert(ctx *base.Context, mainInput *base.OperatorIO, args ResetAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	tempAlert := Alert{Name: args.Name, Category: args.Category, Severity: 5} // just to get the name
	var a AlertWithMetadata
	execTriggers := false
	ns.UpdateTransaction(tempAlert.GetFullName(), func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			a = AlertWithMetadata{
				Alert:   tempAlert,
				Counter: 0,
				First:   time.Time{},
				Last:    time.Time{},
			}
			// after restarts, the alert might be reset, so we need to execute the triggers
			execTriggers = true
		}
		if a.ExpiresInDuration == nil || !a.IsExpired() {
			eTime := time.Now().Sub(a.Last)
			a.ExpiresInDuration = &eTime
			execTriggers = true
		}

		return base.MakeObjectOutput(a)
	}, ctx)
	if execTriggers {
		oc.execTriggers(ctx, a)
	}
	return base.MakeEmptyOutput()
}

// ResetSilence stops ignoring alerts
func (oc *OpAlert) ResetSilence(ctx *base.Context, mainInput *base.OperatorIO, args ResetAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	tempAlert := Alert{Name: args.Name, Category: args.Category, Severity: 5} // just to get the name
	var a AlertWithMetadata
	ns.UpdateTransaction(tempAlert.GetFullName(), func(oi base.OperatorIO) *base.OperatorIO {
		oi.ParseJSON(&a)

		if oi.IsEmpty() {
			return base.MakeOutputError(http.StatusNotFound, "Alert %v does not exist", tempAlert.GetFullName())
		}
		a.SilenceUntil = time.Now()

		return base.MakeObjectOutput(a)
	}, ctx)
	return base.MakeEmptyOutput()
}

type GetAlertArgs struct {
	Severity        *int
	Category        *string
	IncludeSilenced *bool
	IncludeExpired  *bool
	UseTimestamps   *bool
}

func (oc *OpAlert) getAlertList(args GetAlertArgs) ([]AlertWithMetadata, error) {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return make([]AlertWithMetadata, 0), fmt.Errorf("Error getting store: %v", err)
	}
	activeAlerts := make([]AlertWithMetadata, 0)
	for _, entry := range ns.GetAllValues(1000) {
		var a AlertWithMetadata
		entry.ParseJSON(&a)
		if a.IsExpired() && (args.IncludeExpired == nil || *args.IncludeExpired == false) {
			continue
		}
		if args.Severity != nil && a.Severity > *args.Severity {
			continue
		}
		if args.Category != nil && (a.Category == nil || *a.Category != *args.Category) {
			continue
		}
		if a.IsSilenced() && (args.IncludeSilenced == nil || *args.IncludeSilenced == false) {
			continue
		}

		activeAlerts = append(activeAlerts, a)
	}
	return activeAlerts, nil
}

type ReadableAlert struct {
	Name               string
	Category           string
	Desc               string
	Severity           int
	ExpiresInDuration  time.Duration
	Counter            int
	DurationSinceFirst time.Duration
	DurationSinceLast  time.Duration
	SilenceDuration    time.Duration
	ModifiedBy         string
}

func NewReadableAlert(a AlertWithMetadata, modifiedBy string) ReadableAlert {
	r := ReadableAlert{Name: a.Name, Severity: a.Severity, Counter: a.Counter, ModifiedBy: modifiedBy}
	if a.Category != nil {
		r.Category = *a.Category
	}
	if a.Desc != nil {
		r.Desc = *a.Desc
	}
	if a.ExpiresInDuration != nil {
		r.ExpiresInDuration = *a.ExpiresInDuration
	}
	r.DurationSinceFirst = time.Now().Sub(a.First)
	r.DurationSinceLast = time.Now().Sub(a.Last)
	if a.SilenceUntil.After(time.Now()) {
		r.SilenceDuration = a.SilenceUntil.Sub(time.Now())
	}
	return r
}

// GetActiveAlerts returns all active alerts matching the given args
func (oc *OpAlert) GetActiveAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getAlertList(args)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, err.Error())
	}
	return base.MakeObjectOutput(activeAlerts)
}

// GetAlerts returns all alerts matching the given args and returns a list of alerts with readable timestamps and the modified-by-uuid
func (oc *OpAlert) GetAlerts(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error getting store: %v", err)
	}
	alertList := make([]ReadableAlert, 0)
	for _, entry := range ns.GetSearchResultWithMetadata("", "", "", 0, math.MaxInt64) {
		var a AlertWithMetadata
		entry.ParseJSON(&a)
		if a.IsExpired() && (args.IncludeExpired == nil || *args.IncludeExpired == false) {
			continue
		}
		if args.Severity != nil && a.Severity > *args.Severity {
			continue
		}
		if args.Category != nil && (a.Category == nil || *a.Category != *args.Category) {
			continue
		}
		if a.IsSilenced() && (args.IncludeSilenced == nil || *args.IncludeSilenced == false) {
			continue
		}

		alertList = append(alertList, NewReadableAlert(a, entry.GetModifiedBy()))
	}
	return base.MakeObjectOutput(alertList)
}

// GetShortAlertString returns a single string describing all active alerts of a given severity
func (oc *OpAlert) GetShortAlertString(ctx *base.Context, mainInput *base.OperatorIO, args GetAlertArgs) *base.OperatorIO {
	activeAlerts, err := oc.getAlertList(args)
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
			return base.MakePlainOutput(*a.Desc)
		}
		if a.Category == nil {
			return base.MakeSprintfOutput("%v", a.Name)
		}
		return base.MakeSprintfOutput("%v.%v", a.Name, *a.Category)
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
	activeAlerts, err := oc.getAlertList(args)
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
	Name          string
	Category      *string
	IgnoreSilence *bool
}

func (iaa *IsActiveAlertArgs) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(iaa.Category, false)
}

// GetActiveAlert returns alert information if the alert is active
func (oc *OpAlert) GetActiveAlert(ctx *base.Context, mainInput *base.OperatorIO, args IsActiveAlertArgs) *base.OperatorIO {
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, fmt.Sprintf("Error getting store: %v", err))
	}
	tempAlert := Alert{Name: args.Name, Category: args.Category, Severity: 5} // just to get the name
	var a AlertWithMetadata
	oi := ns.GetValue(tempAlert.GetFullName())
	if oi == freepsstore.NotFoundEntry {
		return base.MakeOutputError(http.StatusNotFound, "Alert %v does not exist", tempAlert.GetFullName())
	}
	err = oi.ParseJSON(&a)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "Error parsing alert: %v", err)
	}
	if a.IsExpired() {
		return base.MakeOutputError(http.StatusExpectationFailed, "Alert %v has expired", tempAlert.GetFullName())
	}
	if a.IsSilenced() && (args.IgnoreSilence == nil || *args.IgnoreSilence == false) {
		return base.MakeOutputError(http.StatusExpectationFailed, "Alert %v is silenced", tempAlert.GetFullName())
	}
	return base.MakeObjectOutput(NewReadableAlert(a))
}

// IsActiveAlert returns an empty output if the alert is active
func (oc *OpAlert) IsActiveAlert(ctx *base.Context, mainInput *base.OperatorIO, args IsActiveAlertArgs) *base.OperatorIO {
  r := oc.GetActiveAlert(ctx, mainInput, args)
  if r.IsError() {
    return r
  }
  return base.MakeEmptyOutput()
}
