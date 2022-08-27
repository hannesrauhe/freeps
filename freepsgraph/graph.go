package freepsgraph

import "fmt"

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

type OutputT string

const (
	Unknown OutputT = ""
	Error   OutputT = "error"
	String  OutputT = "string"
)

type OperatorIO struct {
	OutputType OutputT
	HttpCode   uint32
	Ouput      interface{}
}

type Graph struct {
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*OperatorIO
}

type Operator struct {
	OutputType string
}

type GraphEngine struct {
	graphs    map[string]Graph
	operators map[string]Operator
}

func NewGraph(desc *GraphDesc) *Graph {
	g := &Graph{desc: desc}
	g.opOutputs = make(map[string]*OperatorIO)
	return g
}

func MakeOutputError(code uint32, msg string, a ...interface{}) *OperatorIO {
	return &OperatorIO{OutputType: Error, HttpCode: code, fmt.Errorf(msg, a...)}
}

func (io *OperatorIO) GetMap() (map[string]string, error) {
	//TODO(HR) implement
	return make(map[string]string), nil
}

func (io *OperatorIO) IsError() bool {
	return io.OutputType == Error
}

func (op *Operator) Execute(mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	return MakeOutputError(500, "Not Implemented")
}

// Operators: OR, AND, PARALLEL, NOT(?), InputTransform
func (g *Graph) ExecuteOperation(opDesc *GraphOperationDesc, mainArgs map[string]string) *OperatorIO {
	//TODO(HR): what to do if InputFrom/ArgumentsFrom is empty

	input := g.opOutputs[opDesc.InputFrom]
	combinedArgs := make(map[string]string)
	for k, v := range opDesc.Arguments {
		combinedArgs[k] = v
	}
	if opDesc.ArgumentsFrom == ROOT_SYMBOL {
		for k, v := range mainArgs {
			combinedArgs[k] = v
		}
	} else {
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

	op, exists := g.engine.operators[opDesc.Name]
	if exists {
		return op.Execute(combinedArgs, input)
	}
	subGraph, exists := g.engine.graphs[opDesc.Name]
	if exists {
		return subGraph.Execute(combinedArgs, input)
	}
	return MakeOutputError(404, "Neither graph nor operator with name \"%s\" found", opDesc.Name)
}

func (g *Graph) Execute(mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput
	for _, operation := range g.desc.Operations {
		output := g.ExecuteOperation(&operation, mainArgs)
		g.opOutputs[operation.Name] = output
		if output.IsError() {
			return output
		}
	}
	return g.opOutputs[g.desc.OutputFrom]
}
