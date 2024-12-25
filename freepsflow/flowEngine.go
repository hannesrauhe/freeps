package freepsflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

// FlowEngineConfig is the configuration for the FlowEngine
type FlowEngineConfig struct {
	Flows         map[string]FlowDesc
	FlowsFromURL  []string
	FlowsFromFile []string
	AlertDuration time.Duration
}

var DefaultFlowEngineConfig = FlowEngineConfig{FlowsFromFile: []string{}, FlowsFromURL: []string{}, Flows: map[string]FlowDesc{}, AlertDuration: time.Hour}

// FlowEngineMetrics holds the metrics of the flow engine
type FlowEngineMetrics struct {
	OperationExecutions int64
	FlowExecutions      int64
}

// FlowEngine holds all available flows and operators
type FlowEngine struct {
	cr              *utils.ConfigReader
	flows           map[string]*FlowDesc
	operators       map[string]base.FreepsBaseOperator
	hooks           map[string]FlowEngineHook
	metrics         FlowEngineMetrics
	config          FlowEngineConfig
	reloadRequested bool
	flowLock        sync.Mutex
	operatorLock    sync.Mutex
	hookMapLock     sync.Mutex
}

// NewFlowEngine creates the flow engine from the config
func NewFlowEngine(ctx *base.Context, cr *utils.ConfigReader, cancel context.CancelFunc) *FlowEngine {
	ge := &FlowEngine{cr: cr, flows: make(map[string]*FlowDesc), reloadRequested: false}

	ge.operators = make(map[string]base.FreepsBaseOperator)
	ge.operators["flow"] = &OpFlow{ge: ge}
	ge.operators["flowbytag"] = &OpFlowByTag{ge: ge}
	ge.operators["system"] = NewSytemOp(ge, cancel)
	/* backward compatibility to <1.4 */
	ge.operators["graph"] = &OpFlow{ge: ge}
	ge.operators["graphbytag"] = &OpFlowByTag{ge: ge}
	ge.operators["eval"] = &OpEval{}

	ge.hooks = make(map[string]FlowEngineHook)

	// probably deprecated anyhow to start without config
	if cr != nil {
		ge.config = ge.ReadConfig()
		ge.loadStoredAndEmbeddedFlows(ctx)

		g := ge.GetAllFlowDesc()
		addedFlows := make([]string, 0, len(g))
		for n := range g {
			addedFlows = append(addedFlows, n)
		}
	} else {
		ge.config = DefaultFlowEngineConfig
	}

	return ge
}

// getHookMapCopy returns a copy of the hook-map to reduce locking time (hook map does not need to be locked while hook is executed)
func (ge *FlowEngine) getHookMapCopy() map[string]FlowEngineHook {
	ge.hookMapLock.Lock()
	defer ge.hookMapLock.Unlock()
	r := make(map[string]FlowEngineHook)
	for k, v := range ge.hooks {
		if v == nil {
			continue
		}
		r[k] = v
	}
	return r
}

// ReadConfig reads the config from the config reader
func (ge *FlowEngine) ReadConfig() FlowEngineConfig {
	config := DefaultFlowEngineConfig
	if ge.cr != nil {
		err := ge.cr.ReadSectionWithDefaults("flows", &config)
		if err != nil {
			log.Fatal(err)
		}
		ge.cr.WriteBackConfigIfChanged()
		if err != nil {
			log.Print(err)
		}
	}
	return config
}

// ReloadRequested returns true if a reload was requested instead of a restart
func (ge *FlowEngine) ReloadRequested() bool {
	return ge.reloadRequested
}

func (ge *FlowEngine) getFlowDescUnlocked(flowName string) (*FlowDesc, bool) {
	gi, exists := ge.flows[flowName]
	if exists {
		return gi, exists
	}
	return nil, false
}

// CheckFlow checks if the flow is valid
func (ge *FlowEngine) CheckFlow(flowID string) *base.OperatorIO {
	_, o := ge.prepareFlowExecution(nil, flowID)
	return o
}

// GetFlowDesc returns the flow description stored under flowID
func (ge *FlowEngine) GetFlowDesc(flowID string) (*FlowDesc, bool) {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()
	gi, exists := ge.getFlowDescUnlocked(flowID)
	if exists {
		return gi, exists
	}
	return nil, exists
}

// GetCompleteFlowDesc returns the sanitized, validated and complete flow description stored under flowName
func (ge *FlowEngine) GetCompleteFlowDesc(flowID string) (*FlowDesc, error) {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()
	gi, exists := ge.getFlowDescUnlocked(flowID)
	if exists {
		return gi.GetCompleteDesc(flowID, ge)
	}
	return nil, fmt.Errorf("Flow with ID \"%v\" does not exist", flowID)
}

// GetTags returns a map of all used tags TODO(HR): deprecate
func (ge *FlowEngine) GetTags() map[string]string {
	r := map[string]string{}
	for _, d := range ge.GetAllFlowDesc() {
		if d.Tags == nil {
			continue
		}
		for _, t := range d.Tags {
			r[t] = t
		}
	}
	return r
}

// SplitTag returns the tag name and the value of the tag if any (split by the first ":")
func SplitTag(tag string) (string, string) {
	if utils.StringStartsWith(tag, ":") {
		return tag, ""
	}
	tmp := strings.Split(tag, ":")
	if len(tmp) == 1 {
		return tmp[0], ""
	}
	if len(tmp) == 2 {
		return tmp[0], tmp[1]
	}
	if len(tmp) > 2 {
		return tmp[0], strings.Join(tmp[1:], ":")
	}
	return "", ""
}

// GetTagValues matches all Flows with the given tags and returns a slice of all used values for the first given tag, every value occurs only once
func (ge *FlowEngine) GetTagValues(keytagAndOtherTags ...string) []string {
	r := map[string]int{}
	keytag := keytagAndOtherTags[0]
	l := len(keytag)
	if l == 0 {
		return []string{}
	}
	for _, d := range ge.GetFlowDescByTag(keytagAndOtherTags) {
		if d.Tags == nil {
			continue
		}
		for _, t := range d.Tags {
			k, v := SplitTag(t)
			if k == keytag && v != "" {
				r[v] = 1
			}
		}
	}
	resultArray := make([]string, 0, len(r))
	for k := range r {
		resultArray = append(resultArray, k)
	}
	return resultArray
}

// GetTagMap returns a map of all used tags and their values (empty array if no value)
func (ge *FlowEngine) GetTagMap() map[string][]string {
	r := map[string][]string{}
	for _, d := range ge.GetAllFlowDesc() {
		if d.Tags == nil {
			continue
		}
		for _, t := range d.Tags {
			k, v := SplitTag(t)
			if k == "" {
				continue
			}
			if _, exists := r[k]; !exists {
				r[k] = []string{}
			}
			if v == "" {
				continue
			}
			//only append if not yet in list
			for _, e := range r[k] {
				if e == v {
					continue
				}
			}
			r[k] = append(r[k], v)
		}
	}
	// sort arrays in map
	for k := range r {
		sort.Strings(r[k])
	}

	return r
}

// GetAllFlowDesc returns all flows by name
func (ge *FlowEngine) GetAllFlowDesc() map[string]*FlowDesc {
	r := make(map[string]*FlowDesc)
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()

	for n, g := range ge.flows {
		r[n] = g
	}
	return r
}

// GetFlowDescByTag returns the FlowInfo for all Flows that cointain all of the given tags
func (ge *FlowEngine) GetFlowDescByTag(tags []string) map[string]FlowDesc {
	taggroups := [][]string{}
	for _, t := range tags {
		taggroups = append(taggroups, []string{t})
	}
	return ge.GetFlowDescByTagExtended(taggroups...)
}

// GetFlowDescByTagExtended returns the FlowInfo for all Flows that contain at least one tag of each group
func (ge *FlowEngine) GetFlowDescByTagExtended(tagGroups ...[]string) map[string]FlowDesc {
	r := make(map[string]FlowDesc)
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()

	for n, g := range ge.flows {
		if g.HasAtLeastOneTagPerGroup(tagGroups...) {
			r[n] = *g
		}
	}
	return r
}

// AddOperator adds an operator to the flow engine
func (ge *FlowEngine) AddOperator(op base.FreepsBaseOperator) {
	ge.AddOperators([]base.FreepsBaseOperator{op})
}

// AddOperators adds multiple operators to the flow engine
func (ge *FlowEngine) AddOperators(ops []base.FreepsBaseOperator) {
	if ops == nil {
		return
	}
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()

	for _, op := range ops {
		ge.operators[utils.StringToLower(op.GetName())] = op
		h := op.GetHook()
		if h != nil {
			geh, ok := h.(FlowEngineHook)
			if !ok {
				geh = NewFreepsHookWrapper(h)
			}
			ge.AddHook(geh)
		}
	}
}

// HasOperator returns true if this operator is available in the engine
func (ge *FlowEngine) HasOperator(opName string) bool {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	_, exists := ge.operators[utils.StringToLower(opName)]
	return exists
}

// GetOperators returns the list of available operators
func (ge *FlowEngine) GetOperators() []string {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	r := make([]string, 0, len(ge.operators))
	for _, op := range ge.operators {
		r = append(r, op.GetName())
	}
	return r
}

// GetOperator returns the operator with the given name
func (ge *FlowEngine) GetOperator(opName string) base.FreepsBaseOperator {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	op, exists := ge.operators[utils.StringToLower(opName)]
	if !exists {
		return nil
	}
	return op
}

// AddHook adds a hook to the flow engine
func (ge *FlowEngine) AddHook(h FlowEngineHook) {
	ge.hookMapLock.Lock()
	defer ge.hookMapLock.Unlock()
	ge.hooks[h.GetName()] = h
}

// TriggerOnExecuteHooks adds a hook to the flow engine
func (ge *FlowEngine) TriggerOnExecuteHooks(ctx *base.Context, flowName string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		fh, ok := h.(FreepsExecutionHook)
		if !ok {
			continue
		}
		err := fh.OnExecute(ctx, flowName, mainArgs.GetOriginalCaseMapJoined(), mainInput)
		if err != nil {
			upErr := fmt.Errorf("Execution of Hook \"%v\" failed with error: %v", name, err.Error())
			ge.SetSystemAlert(ctx, "ExecuteHook"+name, "system", 3, upErr, &ge.config.AlertDuration)
		}
	}
}

// TriggerOnExecutionFinishedHooks executes hooks when Execution of a flow finishes
func (ge *FlowEngine) TriggerOnExecutionFinishedHooks(ctx *base.Context, flowName string, mainArgs base.FunctionArguments, mainInput *base.OperatorIO) {
	if r := recover(); r != nil {
		logger := ctx.GetLogger()
		err := fmt.Errorf("panic during execution of %v: %v", flowName, r)
		logger.Errorf(err.Error())
		ge.SetSystemAlert(ctx, "Panic", "system", 2, err, &ge.config.AlertDuration)
		return
	}

	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		fh, ok := h.(FreepsExecutionHook)
		if !ok {
			continue
		}
		err := fh.OnExecutionFinished(ctx, flowName, mainArgs.GetOriginalCaseMapJoined(), mainInput)
		if err != nil {
			upErr := fmt.Errorf("Execution of FinishedHook \"%v\" failed with error: %v", name, err.Error())
			ge.SetSystemAlert(ctx, "ExecutionFinishedHook"+name, "system", 3, upErr, &ge.config.AlertDuration)
		}
	}
}

// TriggerOnExecuteOperationHooks executes hooks immediately after an operation was executed
func (ge *FlowEngine) TriggerOnExecuteOperationHooks(ctx *base.Context, input *base.OperatorIO, output *base.OperatorIO, flowName string, od *FlowOperationDesc) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		fh, ok := h.(FreepsExecutionHook)
		if !ok {
			continue
		}
		err := fh.OnExecuteOperation(ctx, input, output, flowName, od)
		if err != nil {
			upErr := fmt.Errorf("Execution of FailedHook \"%v\" failed with error: %v", name, err.Error())
			ge.SetSystemAlert(ctx, "ExecutionErrorHook"+name, "system", 3, upErr, &ge.config.AlertDuration)
		}
	}
}

// TriggerFlowChangedHooks triggers hooks whenever a flow was added or removed
func (ge *FlowEngine) TriggerFlowChangedHooks(ctx *base.Context, addedFlowNames []string, removedFlowNames []string) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		fh, ok := h.(FreepsFlowChangedHook)
		if !ok {
			continue
		}
		err := fh.OnFlowChanged(ctx, addedFlowNames, removedFlowNames)
		if err != nil {
			upErr := fmt.Errorf("Execution of FlowChangedHook \"%v\" failed with error: %v", name, err.Error())
			ge.SetSystemAlert(ctx, "FlowChangedHook"+name, "system", 3, upErr, &ge.config.AlertDuration)
		}
	}
}

// SetSystemAlert triggers the hooks with the same name
func (ge *FlowEngine) SetSystemAlert(ctx *base.Context, name string, category string, severity int, err error, expiresIn *time.Duration) {
	hooks := ge.getHookMapCopy()

	for _, h := range hooks {
		fh, ok := h.(FreepsAlertHook)
		if !ok {
			continue
		}
		err := fh.OnSystemAlert(ctx, name, category, severity, err, expiresIn)
		if err != nil {
			ctx.GetLogger().Errorf("Couldn't set SystemAlert: %v", err.Error())
		}
	}
}

// ResetSystemAlert triggers the hooks with the same name
func (ge *FlowEngine) ResetSystemAlert(ctx *base.Context, name string, category string) {
	hooks := ge.getHookMapCopy()

	for _, h := range hooks {
		fh, ok := h.(FreepsAlertHook)
		if !ok {
			continue
		}
		err := fh.OnResetSystemAlert(ctx, name, category)
		if err != nil {
			ctx.GetLogger().Errorf("Couldn't reset SystemAlert: %v", err.Error())
		}
	}
}

// AddFlow adds a flow from an external source and stores it on disk, after checking if the flow is valid
func (ge *FlowEngine) AddFlow(ctx *base.Context, flowID string, gd FlowDesc, overwrite bool) error {
	// check if flow is valid
	_, err := gd.GetCompleteDesc(flowID, ge)
	if err != nil {
		return err
	}
	defer ge.TriggerFlowChangedHooks(ctx, []string{flowID}, []string{})

	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()
	return ge.addFlowUnderLock(ctx, flowID, gd, true, overwrite)
}

func (ge *FlowEngine) addFlowUnderLock(ctx *base.Context, flowName string, gd FlowDesc, writeToDisk bool, overwrite bool) error {
	oldFlow, ok := ge.flows[flowName]
	if ok {
		if overwrite {
			log.Warnf("Flow \"%v\" already exists (Source \"%v\"), overwriting with new source \"%v\"", flowName, oldFlow.Source, gd.Source)
		} else {
			return fmt.Errorf("Flow \"%v\" already exists, please explicitly delete the flow to continue", flowName)
		}
	}

	if gd.Tags == nil {
		gd.Tags = []string{}
	}
	if writeToDisk {
		fileName := "flows/" + flowName + ".json"
		err := ge.cr.WriteObjectToFile(gd, fileName)
		if err != nil {
			ge.SetSystemAlert(ctx, "FlowWriteError", "system", 2, err, &ge.config.AlertDuration)
			return fmt.Errorf("Error writing flows to file %s: %s", fileName, err.Error())
		}
	}
	ge.flows[flowName] = &gd

	return nil
}

// DeleteFlow removes a flow from the engine and from the storage
func (ge *FlowEngine) DeleteFlow(ctx *base.Context, flowID string) (*FlowDesc, error) {
	if flowID == "" {
		return nil, errors.New("No name given")
	}

	defer ge.TriggerFlowChangedHooks(ctx, []string{}, []string{flowID})

	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()
	/* remove the flow from memory*/
	deletedFlow, exists := ge.flows[flowID]
	if !exists {
		return nil, errors.New("Flow not found")
	}
	delete(ge.flows, flowID)

	fname := "flows/" + flowID + ".json"
	err := ge.cr.RemoveFile(fname)

	return deletedFlow, err
}

// GetMetrics returns the metrics of the flow engine
func (ge *FlowEngine) GetMetrics() FlowEngineMetrics {
	return ge.metrics
}

// StartListening starts all listening operators
func (ge *FlowEngine) StartListening(ctx *base.Context) {
	defer ge.TriggerFlowChangedHooks(ctx, []string{}, []string{})
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()

	for _, op := range ge.operators {
		if op != nil {
			op.StartListening(ctx)
		}
	}
}

// Shutdown should be called for graceful shutdown
func (ge *FlowEngine) Shutdown(ctx *base.Context) {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()

	for _, op := range ge.operators {
		if op != nil {
			ctx.GetLogger().Debugf("Stopping %v", op.GetName())
			op.Shutdown(ctx)
		}
	}
}
