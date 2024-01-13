package freepsgraph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

// GraphEngineConfig is the configuration for the GraphEngine
type GraphEngineConfig struct {
	Graphs         map[string]GraphDesc
	GraphsFromURL  []string
	GraphsFromFile []string
}

var DefaultGraphEngineConfig = GraphEngineConfig{GraphsFromFile: []string{}, GraphsFromURL: []string{}, Graphs: map[string]GraphDesc{}}

// GraphEngine holds all available graphs and operators
type GraphEngine struct {
	cr              *utils.ConfigReader
	graphs          map[string]*GraphDesc
	operators       map[string]base.FreepsBaseOperator
	hooks           map[string]FreepsHook
	reloadRequested bool
	graphLock       sync.Mutex
	operatorLock    sync.Mutex
	hookMapLock     sync.Mutex
}

// NewGraphEngine creates the graph engine from the config
func NewGraphEngine(cr *utils.ConfigReader, cancel context.CancelFunc) *GraphEngine {
	ge := &GraphEngine{cr: cr, graphs: make(map[string]*GraphDesc), reloadRequested: false}

	ge.operators = make(map[string]base.FreepsBaseOperator)
	ge.operators["graph"] = &OpGraph{ge: ge}
	ge.operators["graphbytag"] = &OpGraphByTag{ge: ge}
	ge.operators["system"] = NewSytemOp(ge, cancel)
	ge.operators["eval"] = &OpEval{}

	ge.hooks = make(map[string]FreepsHook)

	if cr != nil {
		ge.loadStoredAndEmbeddedGraphs()
		ge.loadExternalGraphs()

		g := ge.GetAllGraphDesc()
		addedGraphs := make([]string, 0, len(g))
		for n := range g {
			addedGraphs = append(addedGraphs, n)
		}
	}

	return ge
}

// getHookMapCopy returns a copy of the hook-map to reduce locking time (hook map does not need to be locked while hook is executed)
func (ge *GraphEngine) getHookMapCopy() map[string]FreepsHook {
	ge.hookMapLock.Lock()
	defer ge.hookMapLock.Unlock()
	r := make(map[string]FreepsHook)
	for k, v := range ge.hooks {
		if v == nil {
			continue
		}
		r[k] = v
	}
	return r
}

// ReadConfig reads the config from the config reader
func (ge *GraphEngine) ReadConfig() GraphEngineConfig {
	config := DefaultGraphEngineConfig
	if ge.cr != nil {
		err := ge.cr.ReadSectionWithDefaults("graphs", &config)
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
func (ge *GraphEngine) ReloadRequested() bool {
	return ge.reloadRequested
}

func (ge *GraphEngine) getGraphDescUnlocked(graphName string) (*GraphDesc, bool) {
	gi, exists := ge.graphs[graphName]
	if exists {
		return gi, exists
	}
	return nil, false
}

// CheckGraph checks if the graph is valid
func (ge *GraphEngine) CheckGraph(graphName string) *base.OperatorIO {
	_, o := ge.prepareGraphExecution(nil, graphName)
	return o
}

// GetGraphDesc returns the graph description stored under graphName
func (ge *GraphEngine) GetGraphDesc(graphName string) (*GraphDesc, bool) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gi, exists := ge.getGraphDescUnlocked(graphName)
	if exists {
		return gi, exists
	}
	return nil, exists
}

// GetTags returns a map of all used tags TODO(HR): deprecate
func (ge *GraphEngine) GetTags() map[string]string {
	r := map[string]string{}
	for _, d := range ge.GetAllGraphDesc() {
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

// GetTagValues returns a slice of all used values for the given tag
func (ge *GraphEngine) GetTagValues(keytag string) []string {
	r := []string{}
	l := len(keytag)
	if l == 0 {
		return r
	}
	for _, d := range ge.GetAllGraphDesc() {
		if d.Tags == nil {
			continue
		}
		for _, t := range d.Tags {
			k, v := SplitTag(t)
			if k == keytag && v != "" {
				r = append(r, v)
			}
		}
	}
	return r
}

// GetTagMap returns a map of all used tags and their values (empty array if no value)
func (ge *GraphEngine) GetTagMap() map[string][]string {
	r := map[string][]string{}
	for _, d := range ge.GetAllGraphDesc() {
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

// GetAllGraphDesc returns all graphs by name
func (ge *GraphEngine) GetAllGraphDesc() map[string]*GraphDesc {
	r := make(map[string]*GraphDesc)
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	for n, g := range ge.graphs {
		r[n] = g
	}
	return r
}

// GetGraphDescByTag returns the GraphInfo for all Graphs that cointain all of the given tags
func (ge *GraphEngine) GetGraphDescByTag(tags []string) map[string]GraphDesc {
	taggroups := [][]string{}
	for _, t := range tags {
		taggroups = append(taggroups, []string{t})
	}
	return ge.GetGraphDescByTagExtended(taggroups)
}

// GetGraphDescByTagExtended returns the GraphInfo for all Graphs that contain at least one tag of each group
func (ge *GraphEngine) GetGraphDescByTagExtended(tagGroups [][]string) map[string]GraphDesc {
	r := make(map[string]GraphDesc)
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	for n, g := range ge.graphs {
		if g.HasAtLeastOneTagPerGroup(tagGroups) {
			r[n] = *g
		}
	}
	return r
}

// AddOperator adds an operator to the graph engine
func (ge *GraphEngine) AddOperator(op base.FreepsBaseOperator) {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	if op != nil {
		ge.operators[utils.StringToLower(op.GetName())] = op
		h := op.GetHook()
		if h != nil {
			ge.AddHook(h.(FreepsHook))
		}
	}
}

// AddOperators adds multiple operators to the graph engine
func (ge *GraphEngine) AddOperators(ops []base.FreepsBaseOperator) {
	if ops == nil {
		return
	}
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()

	for _, op := range ops {
		ge.operators[utils.StringToLower(op.GetName())] = op
		h := op.GetHook()
		if h != nil {
			ge.AddHook(h.(FreepsHook))
		}
	}
}

// HasOperator returns true if this operator is available in the engine
func (ge *GraphEngine) HasOperator(opName string) bool {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	_, exists := ge.operators[utils.StringToLower(opName)]
	return exists
}

// GetOperators returns the list of available operators
func (ge *GraphEngine) GetOperators() []string {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	r := make([]string, 0, len(ge.operators))
	for _, op := range ge.operators {
		r = append(r, op.GetName())
	}
	return r
}

// GetOperator returns the operator with the given name
func (ge *GraphEngine) GetOperator(opName string) base.FreepsBaseOperator {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	op, exists := ge.operators[utils.StringToLower(opName)]
	if !exists {
		return nil
	}
	return op
}

// AddHook adds a hook to the graph engine
func (ge *GraphEngine) AddHook(h FreepsHook) {
	ge.hookMapLock.Lock()
	defer ge.hookMapLock.Unlock()
	ge.hooks[h.GetName()] = h
}

// TriggerOnExecuteHooks adds a hook to the graph engine
func (ge *GraphEngine) TriggerOnExecuteHooks(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		err := h.OnExecute(ctx, graphName, mainArgs, mainInput)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of Hook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerOnExecuteOperationHooks executes hooks when an operation is executed
func (ge *GraphEngine) TriggerOnExecuteOperationHooks(ctx *base.Context, operationIndexInContext int) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		err := h.OnExecuteOperation(ctx, operationIndexInContext)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of OperationHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerOnExecutionFinishedHooks executes hooks when Execution of a graph finishes
func (ge *GraphEngine) TriggerOnExecutionFinishedHooks(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		err := h.OnExecutionFinished(ctx, graphName, mainArgs, mainInput)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of FinishedHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerOnExecutionErrorHooks executes hooks when Execution of a graph fails
func (ge *GraphEngine) TriggerOnExecutionErrorHooks(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *GraphOperationDesc) {
	hooks := ge.getHookMapCopy()

	for name, h := range hooks {
		err := h.OnExecutionError(ctx, input, err, graphName, od)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of FailedHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerGraphChangedHooks triggers hooks whenever a graph was added or removed
func (ge *GraphEngine) TriggerGraphChangedHooks(addedGraphNames []string, removedGraphNames []string) {
	hooks := ge.getHookMapCopy()

	for _, h := range hooks {
		err := h.OnGraphChanged(addedGraphNames, removedGraphNames)
		if err != nil {
			// ctx.GetLogger().Errorf("Execution of GraphChangedHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// AddGraph adds a graph from an external source and stores it on disk, after checking if the graph is valid
func (ge *GraphEngine) AddGraph(graphID string, gd GraphDesc, overwrite bool) error {
	// check if graph is valid
	_, err := gd.GetCompleteDesc(graphID, ge)
	if err != nil {
		return err
	}
	defer ge.TriggerGraphChangedHooks([]string{}, []string{})

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	return ge.addGraphUnderLock(graphID, gd, true, overwrite)
}

func (ge *GraphEngine) addGraphUnderLock(graphName string, gd GraphDesc, writeToDisk bool, overwrite bool) error {
	oldGraph, ok := ge.graphs[graphName]
	if ok {
		if overwrite {
			log.Warnf("Graph \"%v\" already exists (Source \"%v\"), overwriting with new source \"%v\"", graphName, oldGraph.Source, gd.Source)
		} else {
			return fmt.Errorf("Graph \"%v\" already exists, please explicitly delete the graph to continue", graphName)
		}
	}

	if gd.Tags == nil {
		gd.Tags = []string{}
	}
	if writeToDisk {
		fileName := "graphs/" + graphName + ".json"
		err := ge.cr.WriteObjectToFile(gd, fileName)
		if err != nil {
			return fmt.Errorf("Error writing graphs to file %s: %s", fileName, err.Error())
		}
	}
	ge.graphs[graphName] = &gd

	return nil
}

// DeleteGraph removes a graph from the engine and from the storage
func (ge *GraphEngine) DeleteGraph(graphName string) (*GraphDesc, error) {
	if graphName == "" {
		return nil, errors.New("No name given")
	}

	defer ge.TriggerGraphChangedHooks([]string{}, []string{})

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	/* remove the graph from memory*/
	deletedGraph, exists := ge.graphs[graphName]
	if !exists {
		return nil, errors.New("Graph not found")
	}
	delete(ge.graphs, graphName)

	fname := "graphs/" + graphName + ".json"
	err := ge.cr.RemoveFile(fname)

	return deletedGraph, err
}

// StartListening starts all listening operators
func (ge *GraphEngine) StartListening(ctx *base.Context) {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()

	for _, op := range ge.operators {
		if op != nil {
			op.StartListening(ctx)
		}
	}
}

// Shutdown should be called for graceful shutdown
func (ge *GraphEngine) Shutdown(ctx *base.Context) {
	ge.hookMapLock.Lock()
	defer ge.hookMapLock.Unlock()

	for _, h := range ge.hooks {
		if h != nil {
			h.Shutdown()
		}
	}

	for _, op := range ge.operators {
		if op != nil {
			op.Shutdown(ctx)
		}
	}
}
