package freepsgraph

//GraphEngineConfig is the configuration for the GraphEngine
type GraphEngineConfig struct {
	Graphs        map[string]Graph
	GraphsFromURL string
}

//GraphOperationDesc defines which operator to execute with Arguments and where to take the input from
type GraphOperationDesc struct {
	Name                string
	Operator            string
	Arguments           map[string]string
	InputFrom           string
	UseInputAsArguments bool
}

//GraphDesc contains a number of operations and defines which output to use
type GraphDesc struct {
	Name       string
	OutputFrom     string
	Operations []GraphOperationDesc
}

type OutputT string

const (
	Unknown   OutputT = ""
	Error     OutputT = "error"
	String    OutputT = "string"
)

type OperatorOutput struct {
	OutputType OutputT
	HttpCode   uint32
	Ouput      interface{}
}

type Graph struct {
	desc        *GraphDesc
	engine      *GraphEngine
	opOutputs   map[string]OperatorOutput
	finalOutput OperatorOutput
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
	g.opOutputs = make(map[string]OperatorOutput)
	return g
}

func MakeOutputError(uint32 code, msg string, a ...interface{}) OperatorOutput {
	OperatorOutput{OutputType: Error, HttpCode: 404, fmt.Errorf(msg, a...)}
}

// Operators: OR, AND, PARALLEL, NOT(?), InputToArg, InputTransform
func (g *Graph) ExecuteOperation(op *GraphOperationDesc, args map[string]string) OperatorOutput {
}

func (g *Graph) Execute() OperatorOutput {
	for _, operation := graph.Operations {
		op, exists := g.engine.operators[operation.Name]
		if exists {
			g.opOutputs[operation.Name] = g.ExecuteOperation(op, operation.Arguments)
			continue
		}
		subGraph, exists := g.engine.graphs[operation.Name]
		if exists {
			g.opOutputs[operation.Name] = subGraph.Execute(op, operation.Arguments)
			continue
		}
		return MakeOutputError(404, "Neither graph nor operator with name \"%s\" found", operation.Name)
	}
}
