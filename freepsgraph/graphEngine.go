package freepsgraph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

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
	externalGraphs  map[string]*GraphInfo
	temporaryGraphs map[string]*GraphInfo
	operators       map[string]base.FreepsBaseOperator
	hooks           map[string]FreepsHook
	reloadRequested bool
	graphLock       sync.Mutex
	operatorLock    sync.Mutex
	hookLock        sync.Mutex
}

// NewGraphEngine creates the graph engine from the config
func NewGraphEngine(cr *utils.ConfigReader, cancel context.CancelFunc) *GraphEngine {
	ge := &GraphEngine{cr: cr, externalGraphs: make(map[string]*GraphInfo), temporaryGraphs: make(map[string]*GraphInfo), reloadRequested: false}

	ge.operators = make(map[string]base.FreepsBaseOperator)
	ge.operators["graph"] = &OpGraph{ge: ge}
	ge.operators["graphbytag"] = &OpGraphByTag{ge: ge}
	ge.operators["time"] = &OpTime{}
	ge.operators["curl"] = &OpCurl{}
	ge.operators["system"] = NewSytemOp(ge, cancel)
	ge.operators["eval"] = &OpEval{}

	ge.hooks = make(map[string]FreepsHook)

	if cr != nil {
		var err error
		config := ge.ReadConfig()

		for _, fURL := range config.GraphsFromURL {
			newGraphs := make(map[string]GraphDesc)
			err = cr.ReadObjectFromURL(&newGraphs, fURL)
			if err != nil {
				log.Errorf("Skipping %v, because: %v", fURL, err)
			}
			ge.addExternalGraphsWithSource(newGraphs, "url: "+fURL)
		}
		for _, fName := range config.GraphsFromFile {
			newGraphs := make(map[string]GraphDesc)
			err = cr.ReadObjectFromFile(&newGraphs, fName)
			if err != nil {
				log.Errorf("Skipping %v, because: %v", fName, err)
			}
			ge.addExternalGraphsWithSource(newGraphs, "file: "+fName)
		}

		ge.operators["weather"] = NewWeatherOp(cr)

		if err != nil {
			log.Fatal(err)
		}
	}

	return ge
}

func (ge *GraphEngine) addExternalGraphsWithSource(src map[string]GraphDesc, srcName string) {
	for k, v := range src {
		if v.Tags == nil {
			v.Tags = []string{}
		}
		v.Source = srcName
		ge.externalGraphs[k] = &GraphInfo{Desc: v}
	}
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

// ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	g, o := ge.prepareGraphExecution(ctx, graphName, true)
	if g == nil {
		return o
	}
	ge.TriggerOnExecuteHooks(ctx, graphName, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, graphName, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteOperatorByName executes an operator directly
func (ge *GraphEngine) ExecuteOperatorByName(ctx *base.Context, opName string, fn string, mainArgs map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	name := fmt.Sprintf("OnDemand/%v/%v", opName, fn)
	g, err := NewGraph(ctx, name, &GraphDesc{Operations: []GraphOperationDesc{{Operator: opName, Function: fn}}}, ge)
	if err != nil {
		return base.MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	ge.TriggerOnExecuteHooks(ctx, name, mainArgs, mainInput)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, name, mainArgs, mainInput)
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteGraphByTags executes graphs with given tags
func (ge *GraphEngine) ExecuteGraphByTags(ctx *base.Context, tags []string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	taggroups := [][]string{}
	for _, t := range tags {
		taggroups = append(taggroups, []string{t})
	}
	return ge.ExecuteGraphByTagsExtended(ctx, taggroups, args, input)
}

// ExecuteGraphByTagsExtended executes all graphs that at least one tag of each group
func (ge *GraphEngine) ExecuteGraphByTagsExtended(ctx *base.Context, tagGroups [][]string, args map[string]string, input *base.OperatorIO) *base.OperatorIO {
	if tagGroups == nil || len(tagGroups) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "No tags given")
	}

	// ctx.GetLogger().Infof("Executing graph by tags: %v", tagGroups)

	tg := ge.GetGraphInfoByTagExtended(tagGroups)
	if len(tg) <= 1 {
		for n := range tg {
			return ge.ExecuteGraph(ctx, n, args, input)
		}
		return base.MakeOutputError(404, "No graph with tags found: %v", fmt.Sprint(tagGroups))
	}

	// need to build a temporary graph containing all graphs with matching tags
	op := []GraphOperationDesc{}
	for n := range tg {
		op = append(op, GraphOperationDesc{Name: n, Operator: "graph", Function: n, InputFrom: "_"})
	}
	gd := GraphDesc{Operations: op, Tags: []string{"internal"}}
	name := "ExecuteGraphByTag"

	g, err := NewGraph(ctx, name, &gd, ge)
	if err != nil {
		return base.MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}

	ge.TriggerOnExecuteHooks(ctx, name, args, input)
	defer ge.TriggerOnExecutionFinishedHooks(ctx, name, args, input)
	return g.execute(ctx, args, input)
}

func (ge *GraphEngine) getGraphInfoUnlocked(graphName string) (*GraphInfo, bool) {
	gi, exists := ge.externalGraphs[graphName]
	if exists {
		return gi, exists
	}
	gi, exists = ge.temporaryGraphs[graphName]
	if exists {
		return gi, exists
	}
	return nil, false
}

func (ge *GraphEngine) prepareGraphExecution(ctx *base.Context, graphName string, countExecution bool) (*Graph, *base.OperatorIO) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gi, exists := ge.getGraphInfoUnlocked(graphName)
	if !exists {
		return nil, base.MakeOutputError(404, "No graph with name \"%s\" found", graphName)
	}
	g, err := NewGraph(ctx, graphName, &gi.Desc, ge)
	if err != nil {
		return nil, base.MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	if countExecution {
		gi.LastExecutionTime = time.Now()
		gi.ExecutionCounter++
	}
	return g, base.MakeEmptyOutput()
}

// CheckGraph checks if the graph is valid
func (ge *GraphEngine) CheckGraph(graphName string) *base.OperatorIO {
	_, o := ge.prepareGraphExecution(nil, graphName, false)
	return o
}

// GetGraphDesc returns the graph description stored under graphName
func (ge *GraphEngine) GetGraphDesc(graphName string) (*GraphDesc, bool) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gi, exists := ge.getGraphInfoUnlocked(graphName)
	if exists {
		return &gi.Desc, exists
	}
	return nil, exists
}

// GetTags returns a map of all used tags
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
			if len(t) > l+1 && t[:l] == keytag && t[l] == ':' {
				r = append(r, t[l+1:])
			}
		}
	}
	return r
}

// GetGraphInfo returns the runtime information for the graph with the given Name
func (ge *GraphEngine) GetGraphInfo(graphName string) (GraphInfo, bool) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gi, exists := ge.getGraphInfoUnlocked(graphName)
	if exists {
		return *gi, exists
	}
	return GraphInfo{}, exists
}

// GetAllGraphDesc returns all graphs by name
func (ge *GraphEngine) GetAllGraphDesc() map[string]*GraphDesc {
	r := make(map[string]*GraphDesc)
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	for n, g := range ge.externalGraphs {
		r[n] = &g.Desc
	}
	for n, g := range ge.temporaryGraphs {
		r[n] = &g.Desc
	}
	return r
}

// GetGraphInfoByTag returns the GraphInfo for all Graphs that cointain all of the given tags
func (ge *GraphEngine) GetGraphInfoByTag(tags []string) map[string]GraphInfo {
	taggroups := [][]string{}
	for _, t := range tags {
		taggroups = append(taggroups, []string{t})
	}
	return ge.GetGraphInfoByTagExtended(taggroups)
}

// GetGraphInfoByTagExtended returns the GraphInfo for all Graphs that contain at least one tag of each group
func (ge *GraphEngine) GetGraphInfoByTagExtended(tagGroups [][]string) map[string]GraphInfo {
	r := make(map[string]GraphInfo)
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	for n, g := range ge.externalGraphs {
		if g.Desc.HasAtLeastOneTagPerGroup(tagGroups) {
			r[n] = *g
		}
	}
	for n, g := range ge.temporaryGraphs {
		if g.Desc.HasAtLeastOneTagPerGroup(tagGroups) {
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
	ge.hookLock.Lock()
	defer ge.hookLock.Unlock()
	ge.hooks[h.GetName()] = h
}

// TriggerOnExecuteHooks adds a hook to the graph engine
func (ge *GraphEngine) TriggerOnExecuteHooks(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) {
	ge.hookLock.Lock()
	defer ge.hookLock.Unlock()

	for name, h := range ge.hooks {
		if h == nil {
			continue
		}
		err := h.OnExecute(ctx, graphName, mainArgs, mainInput)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of Hook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerOnExecutionFinishedHooks executes hooks when Execution of a graph finishes
func (ge *GraphEngine) TriggerOnExecutionFinishedHooks(ctx *base.Context, graphName string, mainArgs map[string]string, mainInput *base.OperatorIO) {
	ge.hookLock.Lock()
	defer ge.hookLock.Unlock()

	for name, h := range ge.hooks {
		if h == nil {
			continue
		}
		err := h.OnExecutionFinished(ctx, graphName, mainArgs, mainInput)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of FinishedHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerOnExecutionErrorHooks executes hooks when Execution of a graph fails
func (ge *GraphEngine) TriggerOnExecutionErrorHooks(ctx *base.Context, input *base.OperatorIO, err *base.OperatorIO, graphName string, od *GraphOperationDesc) {
	ge.hookLock.Lock()
	defer ge.hookLock.Unlock()

	for name, h := range ge.hooks {
		if h == nil {
			continue
		}
		err := h.OnExecutionError(ctx, input, err, graphName, od)
		if err != nil {
			ctx.GetLogger().Errorf("Execution of FailedHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// TriggerGraphChangedHooks triggers hooks whenever a graph was added or removed
func (ge *GraphEngine) TriggerGraphChangedHooks(addedGraphNames []string, removedGraphNames []string) {
	ge.hookLock.Lock()
	defer ge.hookLock.Unlock()

	for _, h := range ge.hooks {
		if h == nil {
			continue
		}
		err := h.OnGraphChanged(addedGraphNames, removedGraphNames)
		if err != nil {
			// ctx.GetLogger().Errorf("Execution of GraphChangedHook \"%v\" failed with error: %v", name, err.Error())
		}
	}
}

// AddTemporaryGraph adds a graph to the temporary graph list
func (ge *GraphEngine) AddTemporaryGraph(graphName string, gd *GraphDesc, source string) error {
	_, err := NewGraph(nil, graphName, gd, ge)
	if err != nil {
		return err
	}

	defer ge.TriggerGraphChangedHooks([]string{graphName}, []string{})

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gd.Source = source
	ge.temporaryGraphs[graphName] = &GraphInfo{Desc: *gd}

	return nil
}

// DeleteTemporaryGraph deletes the graph from the temporary graph list
func (ge *GraphEngine) DeleteTemporaryGraph(graphName string) {
	//TODO(HR): figure out changes
	// defere so the lock is released first
	ge.TriggerGraphChangedHooks([]string{}, []string{graphName})

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	delete(ge.temporaryGraphs, graphName)
}

// AddExternalGraph adds a graph to the external graph list and stores it in the config directory
func (ge *GraphEngine) AddExternalGraph(graphName string, gd *GraphDesc, fileName string) error {
	if fileName == "" {
		fileName = "externalGraph_" + graphName + ".json"
	}
	_, err := NewGraph(nil, graphName, gd, ge)
	if err != nil {
		return err
	}
	graphs := make(map[string]GraphDesc)
	graphs[graphName] = *gd
	return ge.AddExternalGraphs(graphs, fileName)
}

// AddExternalGraphs adds a graph to the external graph list and stores it in the config directory
func (ge *GraphEngine) AddExternalGraphs(graphs map[string]GraphDesc, fileName string) error {
	if fileName == "" {
		return errors.New("No filename given")
	}

	//TODO(HR): figure out changes
	// defere so the lock is released first
	defer ge.TriggerGraphChangedHooks([]string{}, []string{})

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	config := ge.ReadConfig()
	exists := false

	existingGraphs := make(map[string]GraphDesc)
	for _, fName := range config.GraphsFromFile {
		if fName == fileName {
			fileName = fName
			err := ge.cr.ReadObjectFromFile(&existingGraphs, fName)
			if err != nil {
				return fmt.Errorf("Error reading graphs from file %s: %s", fName, err.Error())
			}
			exists = true
			break
		}
	}

	if exists {
		for n, g := range graphs {
			existingGraphs[n] = g
		}
		graphs = existingGraphs
	}

	err := ge.cr.WriteObjectToFile(graphs, fileName)
	if err != nil {
		return fmt.Errorf("Error writing graphs to file %s: %s", fileName, err.Error())
	}

	if !exists {
		config.GraphsFromFile = append(config.GraphsFromFile, fileName)
		err := ge.cr.WriteSection("graphs", config, true)
		if err != nil {
			return fmt.Errorf("Error writing config file: %s", err.Error())
		}
	}

	ge.addExternalGraphsWithSource(graphs, fileName)

	// make sure graphs are not in the temporary graph list
	for n := range graphs {
		delete(ge.temporaryGraphs, n)
	}

	return nil
}

// DeleteGraph removes a graph from the engine and from the storage
func (ge *GraphEngine) DeleteGraph(graphName string) error {
	if graphName == "" {
		return errors.New("No name given")
	}

	//TODO(HR): figure out changes
	// defere so the lock is released first
	defer ge.TriggerGraphChangedHooks([]string{}, []string{})

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	/* remove the graph from memory*/
	if _, exists := ge.externalGraphs[graphName]; !exists {
		return nil
	}
	delete(ge.externalGraphs, graphName)

	/* remove graph from file and corresponding file if empty */
	config := ge.ReadConfig()
	checkedFiles := make([]string, 0)

	deleteIndex := -1
	for i, fName := range config.GraphsFromFile {
		existingGraphs := make(map[string]GraphDesc)
		err := ge.cr.ReadObjectFromFile(&existingGraphs, fName)
		if err != nil {
			log.Errorf("Error reading graphs from file %s: %s", fName, err.Error())
		}
		if _, exists := existingGraphs[graphName]; !exists {
			checkedFiles = append(checkedFiles, fName)
			continue
		}
		delete(existingGraphs, graphName)
		if len(existingGraphs) == 0 {
			err = ge.cr.RemoveFile(fName)
			if err != nil {
				log.Errorf("Error deleting file %s: %s", fName, err.Error())
			}
			deleteIndex = i
		} else {
			err = ge.cr.WriteObjectToFile(existingGraphs, fName)
			if err != nil {
				log.Errorf("Error writing to file %s: %s", fName, err.Error())
			}
			checkedFiles = append(checkedFiles, fName)
		}
	}
	config.GraphsFromFile = utils.DeleteElemFromSlice(config.GraphsFromFile, deleteIndex)
	err := ge.cr.WriteSection("graphs", config, true)

	return err
}

// Shutdown should be called for graceful shutdown
func (ge *GraphEngine) Shutdown(ctx *base.Context) {
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
