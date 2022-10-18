package freepsgraph

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

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
	name      string
	desc      *GraphDesc
	engine    *GraphEngine
	opOutputs map[string]*OperatorIO
}

// NewGraph creates a new graph from a graph description
func NewGraph(name string, graphDesc *GraphDesc, ge *GraphEngine) (*Graph, error) {
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
	return &Graph{name: name, desc: &gd, engine: ge, opOutputs: make(map[string]*OperatorIO)}, nil
}

func (g *Graph) execute(logger *log.Entry, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	g.opOutputs[ROOT_SYMBOL] = mainInput
	var failed []string
	for i := 0; i < len(g.desc.Operations); i++ {
		operation := g.desc.Operations[i]
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

func (g *Graph) collectAndReturnOperationError(input *OperatorIO, opDesc *GraphOperationDesc, code uint32, msg string, a ...interface{}) *OperatorIO {
	error := MakeOutputError(code, msg, a...)
	g.engine.executionErrors.AddError(input, error, g.name, opDesc)
	return error
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
			return g.collectAndReturnOperationError(input, opDesc, 404, "Output of \"%s\" cannot be used as arguments, because there is no such output", opDesc.ArgumentsFrom)
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
			return g.collectAndReturnOperationError(input, opDesc, 500, "Output of \"%s\" cannot be used as arguments: %v", opDesc.ArgumentsFrom, err)
		}
		for k, v := range collectedArgs {
			combinedArgs[k] = v
		}
	}

	op, exists := g.engine.operators[opDesc.Operator]
	if exists {
		logger.Debugf("Calling operator \"%v\", Function \"%v\" with arguments \"%v\"", opDesc.Operator, opDesc.Function, combinedArgs)
		output := op.Execute(opDesc.Function, combinedArgs, input)
		if output.IsError() {
			g.engine.executionErrors.AddError(input, output, g.name, opDesc)
		}
		return output
	}
	return g.collectAndReturnOperationError(input, opDesc, 404, "No operator with name \"%s\" found", opDesc.Operator)
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
