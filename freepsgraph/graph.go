package freepsgraph

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/hannesrauhe/freeps/utils"
)

var ROOT_SYMBOL = "_"

//GraphEngineConfig is the configuration for the GraphEngine
type GraphEngineConfig struct {
	Graphs         map[string]GraphDesc
	GraphsFromURL  []string
	GraphsFromFile []string
}

var DefaultGraphEngineConfig = GraphEngineConfig{GraphsFromFile: []string{"graphs.json"}}

//GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name          string
	Operator      string
	Function      string
	Arguments     map[string]string `json:",omitempty"`
	InputFrom     string            `json:",omitempty"`
	ArgumentsFrom string            `json:",omitempty"`
}

//GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	OutputFrom string
	Operations []GraphOperationDesc
}

//Graph is the instance created from a GraphDesc and contains the runtime data
type Graph struct {
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*OperatorIO
}

//GraphEngine holds all available graphs and operators
type GraphEngine struct {
	configGraphs    map[string]GraphDesc
	externalGraphs  map[string]GraphDesc
	temporaryGraphs map[string]GraphDesc
	operators       map[string]FreepsOperator
	reloadRequested bool
	graphLock       sync.Mutex
}

//NewGraphEngine creates the graph engine from the config
func NewGraphEngine(cr *utils.ConfigReader, cancel context.CancelFunc) *GraphEngine {
	ge := &GraphEngine{configGraphs: make(map[string]GraphDesc), externalGraphs: make(map[string]GraphDesc), temporaryGraphs: make(map[string]GraphDesc), reloadRequested: false}
	config := DefaultGraphEngineConfig

	ge.operators = make(map[string]FreepsOperator)
	ge.operators["graph"] = &OpGraph{ge: ge}
	ge.operators["curl"] = &OpCurl{}
	ge.operators["system"] = NewSytemOp(ge, cancel)
	ge.operators["eval"] = &OpEval{}

	if cr != nil {
		err := cr.ReadSectionWithDefaults("graphs", &config)
		if err != nil {
			log.Fatal(err)
		}
		cr.WriteBackConfigIfChanged()
		if err != nil {
			log.Print(err)
		}
		for _, fName := range config.GraphsFromFile {
			err = cr.ReadObjectFromFile(&ge.externalGraphs, fName)
			if err != nil {
				log.Fatal(err)
			}
		}
		tOp := NewTemplateOperator(cr)

		ge.operators["template"] = tOp
		ge.operators["ui"] = NewHTMLUI(tOp.tmc, ge)
	}

	return ge
}

//ReloadRequested returns true if a reload was requested instead of a restart
func (ge *GraphEngine) ReloadRequested() bool {
	return ge.reloadRequested
}

//ExecuteOperatorByName executes an operator directly
func (ge *GraphEngine) ExecuteOperatorByName(opName string, fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g := NewGraph(&GraphDesc{Operations: []GraphOperationDesc{{Operator: opName, Function: fn}}}, ge)
	err := g.prepareAndCheck()
	if err != nil {
		return MakeOutputError(500, "Graph preparation failed: "+err.Error())
	}
	return g.execute(mainArgs, mainInput)
}

//ExecuteGraph executes a graph stored in the engine
func (ge *GraphEngine) ExecuteGraph(graphName string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	gd, exists := ge.GetGraphDesc(graphName)
	if exists {
		g := NewGraph(gd, ge)
		err := g.prepareAndCheck()
		if err != nil {
			return MakeOutputError(500, "Graph preparation failed: "+err.Error())
		}
		return g.execute(mainArgs, mainInput)
	}
	return MakeOutputError(404, "No graph with name \"%s\" found", graphName)
}

//CheckGraph checks if the graph is valid
func (ge *GraphEngine) CheckGraph(graphName string) *OperatorIO {
	gd, exists := ge.GetGraphDesc(graphName)
	if exists {
		g := NewGraph(gd, ge)
		err := g.prepareAndCheck()
		if err != nil {
			return MakeOutputError(500, "Graph preparation failed: "+err.Error())
		}
		return MakeEmptyOutput()
	}
	return MakeOutputError(404, "No graph with name \"%s\" found", graphName)
}

//GetGraphDesc returns the graph description stored under graphName
func (ge *GraphEngine) GetGraphDesc(graphName string) (*GraphDesc, bool) {
	ge.graphLock.Lock()
	defer ge.graphLock.Unlock()
	gd, exists := ge.configGraphs[graphName]
	if exists {
		return &gd, exists
	}
	gd, exists = ge.externalGraphs[graphName]
	if exists {
		return &gd, exists
	}
	gd, exists = ge.temporaryGraphs[graphName]
	if exists {
		return &gd, exists
	}
	return nil, false
}

//GetAllGraphDesc returns all graphs by name
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
	for n, g := range ge.configGraphs {
		r[n] = &g
	}
	return r
}

//HasOperator returns true if the graph is stored in the engine
func (ge *GraphEngine) HasOperator(opName string) bool {
	_, exists := ge.operators[opName]
	return exists
}

//NewGraph creates a new graph from a graph description
func NewGraph(gd *GraphDesc, ge *GraphEngine) *Graph {
	return &Graph{desc: gd, engine: ge, opOutputs: make(map[string]*OperatorIO)}
}

func (g *Graph) prepareAndCheck() error {
	if g.engine == nil {
		return errors.New("GraphEngine not set")
	}
	if g.desc == nil {
		return errors.New("GraphDesc not set")
	}
	if len(g.desc.Operations) == 0 {
		return errors.New("No operations defined")
	}
	outputNames := make(map[string]bool)
	outputNames[ROOT_SYMBOL] = true
	for i := 0; i < len(g.desc.Operations); i++ {
		op := &g.desc.Operations[i]
		if op.Name == ROOT_SYMBOL {
			return errors.New("Operation name cannot be " + ROOT_SYMBOL)
		}
		if op.Name == "" {
			op.Name = fmt.Sprintf("#%d", i)
		}
		if !g.engine.HasOperator(op.Operator) {
			return fmt.Errorf("Operation \"%v\" references unknown operator \"%v\"", op.Operator, op.Name)
		}
		if op.ArgumentsFrom != "" && outputNames[op.ArgumentsFrom] != true {
			return fmt.Errorf("Operation \"%v\" references unknown argumentsFrom \"%v\"", op.Name, op.ArgumentsFrom)
		}
		if op.InputFrom == "" && i == 0 {
			op.InputFrom = ROOT_SYMBOL
		}
		if outputNames[op.InputFrom] != true {
			return fmt.Errorf("Operation \"%v\" references unknown inputFrom \"%v\"", op.Name, op.InputFrom)
		}
		outputNames[op.Name] = true
	}
	if g.desc.OutputFrom == "" {
		if len(g.desc.Operations) == 1 {
			g.desc.OutputFrom = g.desc.Operations[0].Name
		}
	}
	return nil
}

func (g *Graph) execute(mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput

	for i, operation := range g.desc.Operations {
		if _, exist := g.opOutputs[operation.Name]; exist {
			return MakeOutputError(404, "Multiple operations with name \"%s\" found", operation.Name)
		}
		if i == 0 && operation.InputFrom == "" {
			operation.InputFrom = ROOT_SYMBOL
		}
		output := g.executeOperation(&operation, mainArgs)
		g.opOutputs[operation.Name] = output
	}
	if g.desc.OutputFrom == "" {
		return MakeObjectOutput(g.opOutputs)
	}
	return g.opOutputs[g.desc.OutputFrom]
}

func (g *Graph) executeOperation(opDesc *GraphOperationDesc, mainArgs map[string]string) *OperatorIO {
	input := MakeEmptyOutput()
	if opDesc.InputFrom != "" {
		var exists bool
		input, exists = g.opOutputs[opDesc.InputFrom]
		if !exists {
			return MakeOutputError(404, "Output of \"%s\" cannot be used as input for \"%v\", because there is no such output", opDesc.InputFrom, opDesc.Name)
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
		collectedArgs, err := outputToBeArgs.GetMap()
		if err != nil {
			return MakeOutputError(500, "Output of \"%s\" cannot be used as arguments, because it's of type \"%s\"", opDesc.ArgumentsFrom, outputToBeArgs.OutputType)
		}
		for k, v := range collectedArgs {
			combinedArgs[k] = v
		}
	}

	op, exists := g.engine.operators[opDesc.Operator]
	if exists {
		return op.Execute(opDesc.Function, combinedArgs, input)
	}
	return MakeOutputError(404, "No operator with name \"%s\" found", opDesc.Operator)
}
