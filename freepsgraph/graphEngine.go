package freepsgraph

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

// GraphInfo holds the GraphDesc and some runtime info about the graph execution
type GraphInfo struct {
	Desc              GraphDesc
	LastExecutionTime time.Time
	ExecutionCounter  int64
}

// GraphEngine holds all available graphs and operators
type GraphEngine struct {
	cr              *utils.ConfigReader
	externalGraphs  map[string]*GraphInfo
	temporaryGraphs map[string]*GraphInfo
	operators       map[string]FreepsOperator
	hooks           map[string]FreepsHook
	executionErrors *CollectedErrors
	reloadRequested bool
	graphLock       sync.Mutex
	operatorLock    sync.Mutex
}

// NewGraphEngine creates the graph engine from the config
func NewGraphEngine(cr *utils.ConfigReader, cancel context.CancelFunc) *GraphEngine {
	ge := &GraphEngine{cr: cr, externalGraphs: make(map[string]*GraphInfo), temporaryGraphs: make(map[string]*GraphInfo), executionErrors: NewCollectedErrors(100), reloadRequested: false}

	ge.operators = make(map[string]FreepsOperator)
	ge.operators["graph"] = &OpGraph{ge: ge}
	ge.operators["time"] = &OpTime{}
	ge.operators["curl"] = &OpCurl{}
	ge.operators["system"] = NewSytemOp(ge, cancel)
	ge.operators["eval"] = &OpEval{}
	ge.operators["store"] = NewOpStore()
	ge.operators["postgres"] = NewPostgresOp()

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

		ge.operators["fritz"] = NewOpFritz(cr)
		ge.operators["flux"] = NewFluxMod(cr)
		ge.operators["ui"] = NewHTMLUI(cr, ge)

		ge.hooks["postgres"], err = NewPostgressHook(cr)
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

// ExecuteOperatorByName executes an operator directly
func (ge *GraphEngine) ExecuteOperatorByName(ctx *utils.Context, opName string, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	name := fmt.Sprintf("OnDemand/%v/%v", opName, fn)
	g, err := NewGraph(ctx, name, &GraphDesc{Operations: []GraphOperationDesc{{Operator: opName, Function: fn}}}, ge)
	if err != nil {
		return MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	return g.execute(ctx, mainArgs, mainInput)
}

// ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g, o := ge.prepareGraphExecution(ctx, graphName, true)
	if g == nil {
		return o
	}
	for _, h := range ge.hooks {
		if h != nil {
			h.OnExecute(ctx, graphName, mainArgs, mainInput)
		}
	}
	return g.execute(ctx, mainArgs, mainInput)
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

func (ge *GraphEngine) prepareGraphExecution(ctx *utils.Context, graphName string, countExecution bool) (*Graph, *OperatorIO) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gi, exists := ge.getGraphInfoUnlocked(graphName)
	if !exists {
		return nil, MakeOutputError(404, "No graph with name \"%s\" found", graphName)
	}
	g, err := NewGraph(ctx, graphName, &gi.Desc, ge)
	if err != nil {
		return nil, MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	if countExecution {
		gi.LastExecutionTime = time.Now()
		gi.ExecutionCounter++
	}
	return g, MakeEmptyOutput()
}

// CheckGraph checks if the graph is valid
func (ge *GraphEngine) CheckGraph(graphName string) *OperatorIO {
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

// GetGraphInfoByTag returns the GraphInfo for all Graphs with the given tags (logical AND)
func (ge *GraphEngine) GetGraphInfoByTag(tags []string) map[string]GraphInfo {
	r := make(map[string]GraphInfo)
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	for n, g := range ge.externalGraphs {
		if g.Desc.HasTags(tags) {
			r[n] = *g
		}
	}
	for n, g := range ge.temporaryGraphs {
		if g.Desc.HasTags(tags) {
			r[n] = *g
		}
	}
	return r
}

// AddOperator adds an operator to the graph engine
func (ge *GraphEngine) AddOperator(op FreepsOperator) {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	ge.operators[op.GetName()] = op
}

// HasOperator returns true if this operator is available in the engine
func (ge *GraphEngine) HasOperator(opName string) bool {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	_, exists := ge.operators[opName]
	return exists
}

// GetOperators returns the list of available operators
func (ge *GraphEngine) GetOperators() []string {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	r := make([]string, 0, len(ge.operators))
	for n := range ge.operators {
		r = append(r, n)
	}
	return r
}

// GetOperator returns the operator with the given name
func (ge *GraphEngine) GetOperator(opName string) FreepsOperator {
	ge.operatorLock.Lock()
	defer ge.operatorLock.Unlock()
	op, exists := ge.operators[opName]
	if !exists {
		return nil
	}
	return op
}

// AddTemporaryGraph adds a graph to the temporary graph list
func (ge *GraphEngine) AddTemporaryGraph(graphName string, gd *GraphDesc) error {
	_, err := NewGraph(nil, graphName, gd, ge)
	if err != nil {
		return err
	}

	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gd.Source = "temporary"
	ge.temporaryGraphs[graphName] = &GraphInfo{Desc: *gd}
	return nil
}

// DeleteTemporaryGraph deletes the graph from the temporary graph list
func (ge *GraphEngine) DeleteTemporaryGraph(graphName string) {
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

	for _, fName := range config.GraphsFromFile {
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
		} else {
			err = ge.cr.WriteObjectToFile(existingGraphs, fName)
			if err != nil {
				log.Errorf("Error writing to file %s: %s", fName, err.Error())
			}
			checkedFiles = append(checkedFiles, fName)
		}
	}
	return nil
}

// Shutdown should be called for graceful shutdown
func (ge *GraphEngine) Shutdown(ctx *utils.Context) {
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
