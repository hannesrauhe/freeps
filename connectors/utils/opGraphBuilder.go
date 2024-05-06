package freepsutils

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
)

// OpGraphBuilder is the operator to build and modify graphs
type OpGraphBuilder struct {
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperator = &OpGraphBuilder{}

// GraphFromEngineArgs are the arguments for the GraphBuilder function
type GraphFromEngineArgs struct {
	GraphID string
}

// GraphID auggestions returns suggestions for graph names
func (arg *GraphFromEngineArgs) GraphIDSuggestions(m *OpGraphBuilder) map[string]string {
	graphNames := map[string]string{}
	res := m.GE.GetAllGraphDesc()
	for id, gd := range res {
		info, _ := gd.GetCompleteDesc(id, m.GE)
		_, exists := graphNames[info.DisplayName]
		if !exists {
			graphNames[info.DisplayName] = id
		} else {
			graphNames[fmt.Sprintf("%v (ID: %v)", info.DisplayName, id)] = id
		}
	}
	return graphNames
}

// GetGraph returns a graph from the graph engine
func (m *OpGraphBuilder) GetGraph(ctx *base.Context, input *base.OperatorIO, args GraphFromEngineArgs) *base.OperatorIO {
	gd, ok := m.GE.GetGraphDesc(args.GraphID)
	if !ok {
		return base.MakeOutputError(404, "Graph not found in Engine: %v", args.GraphID)
	}
	return base.MakeObjectOutput(gd)
}

// DeleteGraph deletes a graph from the graph engine and stores a backup in the store
func (m *OpGraphBuilder) DeleteGraph(ctx *base.Context, input *base.OperatorIO, args GraphFromEngineArgs) *base.OperatorIO {
	backup, err := m.GE.DeleteGraph(ctx, args.GraphID)
	if backup != nil {
		freepsstore.StoreGraph("deleted_"+args.GraphID, *backup, ctx)
	}
	if err != nil {
		return base.MakeOutputError(400, "Could not delete graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

// GraphFromStoreArgs are the arguments for the GraphBuilder function
type GraphFromStoreArgs struct {
	GraphName       string
	CreateIfMissing *bool
}

// GraphNameSuggestions returns suggestions for graph names
func (arg *GraphFromStoreArgs) GraphNameSuggestions(m *OpGraphBuilder) []string {
	graphNames := []string{}
	res := freepsstore.GetGraphStore().GetAllValues(30)
	for name := range res {
		graphNames = append(graphNames, name)
	}
	return graphNames
}

func (m *OpGraphBuilder) buildDefaultOperation() freepsgraph.GraphOperationDesc {
	return freepsgraph.GraphOperationDesc{
		Operator:  "system",
		Function:  "noop",
		Arguments: map[string]string{},
	}
}

// GetGraphFromStore returns a graph from the store
func (m *OpGraphBuilder) GetGraphFromStore(ctx *base.Context, input *base.OperatorIO, args GraphFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph(args.GraphName)
	if err != nil {
		gdFromEngine, exists := m.GE.GetGraphDesc(args.GraphName)
		if !exists {
			if args.CreateIfMissing == nil || !*args.CreateIfMissing {
				return base.MakeOutputError(404, "Graph not found in store: %v", err)
			}
			gd = freepsgraph.GraphDesc{
				Operations: []freepsgraph.GraphOperationDesc{
					m.buildDefaultOperation(),
				},
			}
		} else {
			gd = *gdFromEngine
		}
		freepsstore.StoreGraph(args.GraphName, gd, ctx)
	}
	return base.MakeObjectOutput(gd)
}

// SetOperationArgs sets the fields of an operation given by the number in a graph in the store
type SetOperationArgs struct {
	GraphName       string
	OperationNumber int
	Operator        *string
	Function        *string
	ArgumentName    *string
	ArgumentValue   *string
}

// SetOperation sets the fields of an operation given by the number in a graph in the store
func (m *OpGraphBuilder) SetOperation(ctx *base.Context, input *base.OperatorIO, args SetOperationArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph(args.GraphName)
	if err != nil {
		return base.MakeOutputError(404, "Graph not found in store: %v", err)
	}
	if args.OperationNumber < 0 || args.OperationNumber > len(gd.Operations) {
		return base.MakeOutputError(400, "Invalid operation number")
	}
	if args.OperationNumber == len(gd.Operations) {
		gd.Operations = append(gd.Operations, m.buildDefaultOperation())
	}

	if args.Operator != nil {
		gd.Operations[args.OperationNumber].Operator = *args.Operator
	}
	if args.Function != nil {
		gd.Operations[args.OperationNumber].Function = *args.Function
	}
	if args.ArgumentName != nil {
		if args.ArgumentValue == nil {
			return base.MakeOutputError(400, "Argument value is missing")
		}
		gd.Operations[args.OperationNumber].Arguments[*args.ArgumentName] = *args.ArgumentValue
	}
	freepsstore.StoreGraph(args.GraphName, gd, ctx)
	return base.MakeEmptyOutput()
}

// RestoreDeletedGraphFromStore restores a graph from the backup in store
func (m *OpGraphBuilder) RestoreDeletedGraphFromStore(ctx *base.Context, input *base.OperatorIO, args GraphFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph("deleted_" + args.GraphName)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore graph: %v", err)
	}
	err = m.GE.AddGraph(ctx, args.GraphName, gd, false)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore graph: %v", err)
	}
	return base.MakeEmptyOutput()
}

// ExecuteGraphFromStore executes a graph after loading it from the store
func (m *OpGraphBuilder) ExecuteGraphFromStore(ctx *base.Context, input *base.OperatorIO, args GraphFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph(args.GraphName)
	if err != nil {
		return base.MakeOutputError(404, "Graph not found in store: %v", err)
	}
	return m.GE.ExecuteAdHocGraph(ctx, "ExecuteFromStore/"+args.GraphName, gd, base.MakeEmptyFunctionArguments(), input)
}

// AddGraph adds a graph to the graph engine (unsused)
// func (m *OpGraphBuilder) AddGraph(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
// 	if !input.IsFormData() {
// 		return base.MakeOutputError(http.StatusBadRequest, "Invalid input format")
// 	}

// 	formdata, err := input.ParseFormData()
// 	if err != nil {
// 		return base.MakeOutputError(http.StatusBadRequest, "Invalid form data: %v", err)
// 	}

// 	graphName := formdata.Get("GraphName")
// 	if graphName == "" {
// 		return base.MakeOutputError(http.StatusBadRequest, "Graph name is missing")
// 	}
// 	overwrite, _ := utils.ConvertToBool(formdata.Get("Overwrite"))
// 	save, _ := utils.ConvertToBool(formdata.Get("SaveGraph"))

// 	gd := freepsgraph.GraphDesc{}
// 	err = json.Unmarshal([]byte(formdata.Get("GraphJSON")), &gd)
// 	if err != nil {
// 		return base.MakeOutputError(http.StatusBadRequest, "Invalid graph JSON: %v", err)
// 	}

// 	if !save {
// 		output := freepsstore.StoreGraph("added_"+graphName, gd, ctx)
// 		return output
// 	} else {
// 		err = m.GE.AddGraph(graphName, gd, overwrite)
// 		if err != nil {
// 			return base.MakeOutputError(400, "Could not add graph: %v", err)
// 		}
// 	}
// 	return base.MakeEmptyOutput()
// }
