package freepsgraph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

const ROOT_SYMBOL = "_"

// GraphEngineConfig is the configuration for the GraphEngine
type GraphEngineConfig struct {
	Graphs         map[string]GraphDesc
	GraphsFromURL  []string
	GraphsFromFile []string
}

var DefaultGraphEngineConfig = GraphEngineConfig{GraphsFromFile: []string{}, GraphsFromURL: []string{}, Graphs: map[string]GraphDesc{}}

// GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name          string `json:",omitempty"`
	Operator      string
	Function      string
	Arguments     map[string]string `json:",omitempty"`
	InputFrom     string            `json:",omitempty"`
	ArgumentsFrom string            `json:",omitempty"`
}

// GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	Tags       []string
	OutputFrom string
	Operations []GraphOperationDesc
}

// Graph is the instance created from a GraphDesc and contains the runtime data
type Graph struct {
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*OperatorIO
}

// GraphEngine holds all available graphs and operators
type GraphEngine struct {
	cr              *utils.ConfigReader
	externalGraphs  map[string]GraphDesc
	temporaryGraphs map[string]GraphDesc
	operators       map[string]FreepsOperator
	reloadRequested bool
	graphLock       sync.Mutex
}

// NewGraphEngine creates the graph engine from the config
func NewGraphEngine(cr *utils.ConfigReader, cancel context.CancelFunc) *GraphEngine {
	ge := &GraphEngine{cr: cr, externalGraphs: make(map[string]GraphDesc), temporaryGraphs: make(map[string]GraphDesc), reloadRequested: false}

	ge.operators = make(map[string]FreepsOperator)
	ge.operators["graph"] = &OpGraph{ge: ge}
	ge.operators["curl"] = &OpCurl{}
	ge.operators["system"] = NewSytemOp(ge, cancel)
	ge.operators["eval"] = &OpEval{}
	ge.operators["mutt"] = &OpMutt{}
	ge.operators["store"] = NewOpStore()
	ge.operators["raspistill"] = &OpRaspistill{}

	if cr != nil {
		var err error
		config := ge.ReadConfig()

		addExternalGraphsWithTag := func(src map[string]GraphDesc, tag string) {
			for k, v := range src {
				if v.Tags == nil {
					v.Tags = []string{}
				}
				v.Tags = append(v.Tags, tag)
				ge.externalGraphs[k] = v
			}
		}

		for _, fURL := range config.GraphsFromURL {
			newGraphs := make(map[string]GraphDesc)
			err = cr.ReadObjectFromURL(&newGraphs, fURL)
			if err != nil {
				log.Fatal(err)
			}
			addExternalGraphsWithTag(newGraphs, "url: "+fURL)
		}
		for _, fName := range config.GraphsFromFile {
			newGraphs := make(map[string]GraphDesc)
			err = cr.ReadObjectFromFile(&newGraphs, fName)
			if err != nil {
				log.Fatal(err)
			}
			addExternalGraphsWithTag(newGraphs, "file: "+fName)
		}
		tOp := NewTemplateOperator(ge, cr)

		ge.operators["template"] = tOp
		ge.operators["fritz"] = NewOpFritz(cr)
		ge.operators["flux"] = NewFluxMod(cr)
		ge.operators["telegram"] = NewTelegramBot(cr)
		ge.operators["ui"] = NewHTMLUI(cr, ge)
		ge.operators["mqtt"] = NewMQTTOp(cr)
	}

	return ge
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
func (ge *GraphEngine) ExecuteOperatorByName(logger log.FieldLogger, opName string, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g, err := NewGraph(&GraphDesc{Operations: []GraphOperationDesc{{Operator: opName, Function: fn}}}, ge)
	if err != nil {
		return MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	dlogger := logger.WithFields(log.Fields{"graph": "direct", "operator": opName, "function": fn})
	return g.execute(dlogger, mainArgs, mainInput)
}

// ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(graphName string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	logger := log.WithFields(log.Fields{"graph": graphName})
	gd, exists := ge.GetGraphDesc(graphName)
	if exists {
		g, err := NewGraph(gd, ge)
		if err != nil {
			return MakeOutputError(500, "Graph preparation failed: "+err.Error())
		}
		return g.execute(logger, mainArgs, mainInput)
	}
	return MakeOutputError(404, "No graph with name \"%s\" found", graphName)
}

// CheckGraph checks if the graph is valid
func (ge *GraphEngine) CheckGraph(graphName string) *OperatorIO {
	gd, exists := ge.GetGraphDesc(graphName)
	if exists {
		_, err := NewGraph(gd, ge)
		if err != nil {
			return MakeOutputError(500, "Graph preparation failed: "+err.Error())
		}
		return MakeEmptyOutput()
	}
	return MakeOutputError(404, "No graph with name \"%s\" found", graphName)
}

// GetGraphDesc returns the graph description stored under graphName
func (ge *GraphEngine) GetGraphDesc(graphName string) (*GraphDesc, bool) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gd, exists := ge.externalGraphs[graphName]
	if exists {
		return &gd, exists
	}
	gd, exists = ge.temporaryGraphs[graphName]
	if exists {
		return &gd, exists
	}
	return nil, false
}

// GetAllGraphDesc returns all graphs by name
func (ge *GraphEngine) GetAllGraphDesc() map[string]*GraphDesc {
	r := make(map[string]*GraphDesc)
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()

	for n, g := range ge.externalGraphs {
		r[n] = &g
	}
	for n, g := range ge.temporaryGraphs {
		r[n] = &g
	}
	return r
}

// HasOperator returns true if this operator is available in the engine
func (ge *GraphEngine) HasOperator(opName string) bool {
	_, exists := ge.operators[opName]
	return exists
}

// GetOperators returns the list of available operators
func (ge *GraphEngine) GetOperators() []string {
	r := make([]string, 0, len(ge.operators))
	for n := range ge.operators {
		r = append(r, n)
	}
	return r
}

// GetOperator returns the operator with the given name
func (ge *GraphEngine) GetOperator(opName string) FreepsOperator {
	op, exists := ge.operators[opName]
	if !exists {
		return nil
	}
	return op
}

// AddTemporaryGraph adds a graph to the temporary graph list
func (ge *GraphEngine) AddTemporaryGraph(graphName string, gd *GraphDesc) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gd.Tags = append(gd.Tags, "temporary")
	ge.temporaryGraphs[graphName] = *gd
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

	err = ge.cr.ReadObjectFromFile(&ge.externalGraphs, fileName)
	if err != nil {
		return fmt.Errorf("Error re-reading graphs from file %s: %s", fileName, err.Error())
	}

	// make sure graphs are not in the temporary graph list
	for n := range graphs {
		delete(ge.temporaryGraphs, n)
	}
	return nil
}

// NewGraph creates a new graph from a graph description
func NewGraph(graphDesc *GraphDesc, ge *GraphEngine) (*Graph, error) {
	if ge == nil {
		return nil, errors.New("GraphEngine not set")
	}
	if graphDesc == nil {
		return nil, errors.New("GraphDesc not set")
	}
	if len(graphDesc.Operations) == 0 {
		return nil, errors.New("No operations defined")
	}
	gd := GraphDesc{OutputFrom: graphDesc.OutputFrom, Operations: make([]GraphOperationDesc, len(graphDesc.Operations))}

	outputNames := make(map[string]bool)
	outputNames[ROOT_SYMBOL] = true
	// create a copy of each operation and add it to the graph
	for i, op := range graphDesc.Operations {
		if op.Name == ROOT_SYMBOL {
			return nil, errors.New("Operation name cannot be " + ROOT_SYMBOL)
		}
		if outputNames[op.Name] {
			return nil, errors.New("Operation name " + op.Name + " is used multiple times")
		}
		if op.Name == "" {
			op.Name = fmt.Sprintf("#%d", i)
		}
		if !ge.HasOperator(op.Operator) {
			return nil, fmt.Errorf("Operation \"%v\" references unknown operator \"%v\"", op.Operator, op.Name)
		}
		if op.ArgumentsFrom != "" && outputNames[op.ArgumentsFrom] != true {
			return nil, fmt.Errorf("Operation \"%v\" references unknown argumentsFrom \"%v\"", op.Name, op.ArgumentsFrom)
		}
		if op.InputFrom == "" && i == 0 {
			op.InputFrom = ROOT_SYMBOL
		}
		if op.InputFrom != "" && outputNames[op.InputFrom] != true {
			return nil, fmt.Errorf("Operation \"%v\" references unknown inputFrom \"%v\"", op.Name, op.InputFrom)
		}
		outputNames[op.Name] = true
		gd.Operations[i] = op

		// op.args are not copied, because they aren't modified during execution
	}
	if graphDesc.OutputFrom == "" {
		if len(graphDesc.Operations) == 1 {
			gd.OutputFrom = gd.Operations[0].Name
		}
	} else if outputNames[graphDesc.OutputFrom] != true {
		return nil, fmt.Errorf("Graph references unknown outputFrom \"%v\"", graphDesc.OutputFrom)
	}
	return &Graph{desc: &gd, engine: ge, opOutputs: make(map[string]*OperatorIO)}, nil
}

func (g *Graph) execute(logger *log.Entry, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput
	var failed []string
	for _, operation := range g.desc.Operations {
		output := g.executeOperation(logger, &operation, mainArgs)
		logger.Debugf("Operation \"%s\" finished with output \"%v\"", operation.Name, output.ToString())
		g.opOutputs[operation.Name] = output
		if output.IsError() {
			failed = append(failed, operation.Name)
		}
	}
	if len(failed) > 0 {
		logger.Errorf("The following operations failed: %v", failed)
	}
	if g.desc.OutputFrom == "" {
		return MakeObjectOutput(g.opOutputs)
	}
	if g.opOutputs[g.desc.OutputFrom] == nil {
		logger.Errorf("Output from \"%s\" not found", g.desc.OutputFrom)
		return MakeObjectOutput(g.opOutputs)
	}
	return g.opOutputs[g.desc.OutputFrom]
}

func (g *Graph) executeOperation(logger *log.Entry, opDesc *GraphOperationDesc, mainArgs map[string]string) *OperatorIO {
	input := MakeEmptyOutput()
	if opDesc.InputFrom != "" {
		input = g.opOutputs[opDesc.InputFrom]
		if input.IsError() {
			// reduce logging of eval-related "errors"
			if input.HTTPCode != http.StatusExpectationFailed {
				logger.Debugf("Not executing executing operation \"%v\", because \"%v\" returned an error", opDesc.Name, opDesc.InputFrom)
			}
			return input
		}
	}
	combinedArgs := make(map[string]string)
	if opDesc.Arguments != nil {
		for k, v := range opDesc.Arguments {
			combinedArgs[k] = v
		}
	}
	for k, v := range mainArgs {
		combinedArgs[k] = v
	}

	if opDesc.ArgumentsFrom != "" {
		outputToBeArgs, exists := g.opOutputs[opDesc.ArgumentsFrom]
		if !exists {
			return MakeOutputError(404, "Output of \"%s\" cannot be used as arguments, because there is no such output", opDesc.ArgumentsFrom)
		}
		if outputToBeArgs.IsError() {
			// reduce logging of eval-related "errors"
			if outputToBeArgs.HTTPCode != http.StatusExpectationFailed {
				logger.Debugf("Not executing executing operation \"%v\", because \"%v\" returned an error", opDesc.Name, opDesc.InputFrom)
			}
			return input
		}
		collectedArgs, err := outputToBeArgs.GetArgsMap()
		if err != nil {
			return MakeOutputError(500, "Output of \"%s\" cannot be used as arguments: %v", opDesc.ArgumentsFrom, err)
		}
		for k, v := range collectedArgs {
			combinedArgs[k] = v
		}
	}

	op, exists := g.engine.operators[opDesc.Operator]
	if exists {
		logger.Debugf("Calling operator \"%v\", Function \"%v\" with arguments \"%v\"", opDesc.Operator, opDesc.Function, combinedArgs)
		output := op.Execute(opDesc.Function, combinedArgs, input)
		return output
	}
	return MakeOutputError(404, "No operator with name \"%s\" found", opDesc.Operator)
}

func (g *Graph) ToDot(gd *GraphDesc) string {
	var s strings.Builder
	s.WriteString("digraph G {")
	s.WriteString("\nArguments")
	s.WriteString("\nInput")
	s.WriteString("\nOutput")
	for _, node := range gd.Operations {
		v := utils.ClearString(node.Name)
		argsF := "Arguments"
		if node.ArgumentsFrom != "" {
			if node.ArgumentsFrom == ROOT_SYMBOL {
				argsF = "Input"
			} else {
				argsF = utils.ClearString(node.ArgumentsFrom)
			}
		}
		s.WriteString("\n" + v)
		s.WriteString("\n" + argsF + "->" + v)

		if node.InputFrom != "" {
			inputF := "Input"
			if node.InputFrom != ROOT_SYMBOL {
				inputF = utils.ClearString(node.InputFrom)
			}
			s.WriteString("\n" + inputF + "->" + v + " [style=dashed]")
		}
	}
	OutputFrom := utils.ClearString(gd.Operations[len(gd.Operations)-1].Name)
	if gd.OutputFrom != "" {
		OutputFrom = utils.ClearString(gd.OutputFrom)
	}
	s.WriteString("\n" + OutputFrom + "->Output [style=dashed]")

	s.WriteString("\n}")
	return s.String()
}
