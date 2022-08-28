package freepsgraph

import (
	"github.com/hannesrauhe/freeps/utils"
)

var ROOT_SYMBOL = "_"

//GraphEngineConfig is the configuration for the GraphEngine
type GraphEngineConfig struct {
	Graphs        map[string]Graph
	GraphsFromURL string
}

//GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name          string
	Operator      string
	Function      string
	Arguments     map[string]string
	InputFrom     string
	ArgumentsFrom string
}

//GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	Name       string
	OutputFrom string
	Operations []GraphOperationDesc
}

type Graph struct {
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*OperatorIO
}

type GraphEngine struct {
	graphs    map[string]Graph
	operators map[string]FreepsOperator
}

func NewGraphEngine(cr *utils.ConfigReader) *GraphEngine {
	ops := make(map[string]FreepsOperator)
	ops["template"] = NewTemplateOperator(cr)
	return &GraphEngine{graphs: make(map[string]Graph), operators: ops}
}

func (ge *GraphEngine) ExecuteOperatorByName(opName string, fn string, mainArgs map[string]string) *OperatorIO {
	g := NewGraph(nil, ge)
	g.opOutputs[ROOT_SYMBOL] = nil
	return g.ExecuteOperation(&GraphOperationDesc{Name: "#0", Operator: opName, Function: fn, InputFrom: ROOT_SYMBOL}, mainArgs)
}

func NewGraph(desc *GraphDesc, engine *GraphEngine) *Graph {
	g := &Graph{desc: desc, engine: engine}
	g.opOutputs = make(map[string]*OperatorIO)
	return g
}

func (g *Graph) ExecuteOperation(opDesc *GraphOperationDesc, mainArgs map[string]string) *OperatorIO {
	//TODO(HR): what to do if InputFrom/ArgumentsFrom is empty

	input, exists := g.opOutputs[opDesc.InputFrom]
	if !exists {
		return MakeOutputError(404, "Output of \"%s\" cannot be used as input for \"%v\", because there is no such output", opDesc.InputFrom, opDesc.Name)
	}
	combinedArgs := make(map[string]string)
	if opDesc.Arguments != nil {
		for k, v := range opDesc.Arguments {
			combinedArgs[k] = v
		}
	}
	if opDesc.ArgumentsFrom == ROOT_SYMBOL {
		for k, v := range mainArgs {
			combinedArgs[k] = v
		}
	} else if opDesc.ArgumentsFrom != "" {
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

func (g *Graph) Execute(mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput

	for _, operation := range g.desc.Operations {
		_, exist := g.opOutputs[operation.Name]
		if exist {
			return MakeOutputError(404, "Multiple operations with name \"%s\" found", operation.Name)
		}
		output := g.ExecuteOperation(&operation, mainArgs)
		g.opOutputs[operation.Name] = output
		if output.IsError() {
			return output
		}
	}
	if g.desc.OutputFrom == "" {
		lastOperation := g.desc.Operations[len(g.desc.Operations)-1]
		return g.opOutputs[lastOperation.Name]
	}
	return g.opOutputs[g.desc.OutputFrom]
}
