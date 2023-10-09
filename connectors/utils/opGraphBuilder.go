package freepsutils

import (
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
	GraphName string
}

// GraphNameSuggestions returns suggestions for graph names
func (arg *GraphFromEngineArgs) GraphNameSuggestions(m *OpGraphBuilder) []string {
	graphNames := []string{}
	res := m.GE.GetAllGraphDesc()
	for name := range res {
		graphNames = append(graphNames, name)
	}
	return graphNames
}

// GetGraph returns a graph from the graph engine
func (m *OpGraphBuilder) GetGraph(ctx *base.Context, input *base.OperatorIO, args GraphFromEngineArgs) *base.OperatorIO {
	gd, ok := m.GE.GetGraphDesc(args.GraphName)
	if !ok {
		return base.MakeOutputError(404, "Graph not found in Engine: %v", args.GraphName)
	}
	return base.MakeObjectOutput(gd)
}

// DeleteGraph deletes a graph from the graph engine and stores a backup in the store
func (m *OpGraphBuilder) DeleteGraph(ctx *base.Context, input *base.OperatorIO, args GraphFromEngineArgs) *base.OperatorIO {
	backup, err := m.GE.DeleteGraph(args.GraphName)
	if backup != nil {
		freepsstore.StoreGraph("deleted_"+args.GraphName, *backup, ctx.GetID())
	}
	if err != nil {
		return base.MakeOutputError(400, "Could not delete graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

// GraphFromStoreArgs are the arguments for the GraphBuilder function
type GraphFromStoreArgs struct {
	GraphName string
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

// GetGraphFromStore returns a graph from the store
func (m *OpGraphBuilder) GetGraphFromStore(ctx *base.Context, input *base.OperatorIO, args GraphFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph(args.GraphName)
	if err != nil {
		return base.MakeOutputError(404, "Graph not found in store: %v", err)
	}
	return base.MakeObjectOutput(gd)
}

// RestoreDeletedGraphFromStore restores a graph from the backup in store
func (m *OpGraphBuilder) RestoreDeletedGraphFromStore(ctx *base.Context, input *base.OperatorIO, args GraphFromStoreArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph("deleted_" + args.GraphName)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore graph: %v", err)
	}
	err = m.GE.AddGraph(args.GraphName, gd)
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
	return m.GE.ExecuteAdHocGraph(ctx, "ExecuteFromStore/"+args.GraphName, gd, make(map[string]string), input)
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
// 		output := freepsstore.StoreGraph("added_"+graphName, gd, ctx.GetID())
// 		return output
// 	} else {
// 		err = m.GE.AddGraph(graphName, gd, overwrite)
// 		if err != nil {
// 			return base.MakeOutputError(400, "Could not add graph: %v", err)
// 		}
// 	}
// 	return base.MakeEmptyOutput()
// }
