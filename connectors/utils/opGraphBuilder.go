package freepsutils

import (
	"encoding/json"
	"net/http"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// OpGraphBuilder is the operator to build and modify graphs
type OpGraphBuilder struct {
	GE *freepsgraph.GraphEngine
}

var _ base.FreepsOperator = &OpGraphBuilder{}

// AddGraph adds a graph to the graph engine
func (m *OpGraphBuilder) AddGraph(ctx *base.Context, input *base.OperatorIO) *base.OperatorIO {
	if !input.IsFormData() {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid input format")
	}

	formdata, err := input.ParseFormData()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid form data: %v", err)
	}

	graphName := formdata.Get("GraphName")
	if graphName == "" {
		return base.MakeOutputError(http.StatusBadRequest, "Graph name is missing")
	}
	overwrite, _ := utils.ConvertToBool(formdata.Get("Overwrite"))
	save, _ := utils.ConvertToBool(formdata.Get("SaveGraph"))

	gd := freepsgraph.GraphDesc{}
	err = json.Unmarshal([]byte(formdata.Get("GraphJSON")), &gd)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid graph JSON: %v", err)
	}

	if !save {
		output := freepsstore.StoreGraph("added_"+graphName, gd, ctx.GetID())
		return output
	} else {
		err = m.GE.AddGraph(graphName, gd, overwrite)
		if err != nil {
			return base.MakeOutputError(400, "Could not add graph: %v", err)
		}
	}
	return base.MakeEmptyOutput()
}

// DeleteGraphArgs are the arguments for the GraphBuilder function
type DeleteGraphArgs struct {
	GraphName string
}

// DeleteGraph deletes a graph from the graph engine and stores a backup in the store
func (m *OpGraphBuilder) DeleteGraph(ctx *base.Context, input *base.OperatorIO, args DeleteGraphArgs) *base.OperatorIO {
	backup, err := m.GE.DeleteGraph(args.GraphName)
	if backup != nil {
		freepsstore.StoreGraph("deleted_"+args.GraphName, *backup, ctx.GetID())
	}
	if err != nil {
		return base.MakeOutputError(400, "Could not delete graph: %v", err)
	}

	return base.MakeEmptyOutput()
}

// RestoreDeletedGraphArgs are the arguments for the GraphBuilder function
type RestoreDeletedGraphArgs struct {
	GraphName string
}

// RestoreDeletedGraph restores a graph from the backup in store
func (m *OpGraphBuilder) RestoreDeletedGraph(ctx *base.Context, input *base.OperatorIO, args RestoreDeletedGraphArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph("deleted_" + args.GraphName)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore graph: %v", err)
	}
	err = m.GE.AddGraph(args.GraphName, gd, false)
	if err != nil {
		return base.MakeOutputError(400, "Could not restore graph: %v", err)
	}
	return base.MakeEmptyOutput()
}

// GetGraphArgs are the arguments for the GraphBuilder function
type GetGraphArgs struct {
	GraphName string
	FromStore *bool
}

// GetGraph returns a graph from the graph engine
func (m *OpGraphBuilder) GetGraph(ctx *base.Context, input *base.OperatorIO, args GetGraphArgs) *base.OperatorIO {
	gd, ok := m.GE.GetGraphDesc(args.GraphName)
	if !ok {
		if args.FromStore != nil && *args.FromStore {
			gd, err := freepsstore.GetGraph(args.GraphName)
			if err != nil {
				return base.MakeOutputError(404, "Graph not found in store: %v", err)
			}
			return base.MakeObjectOutput(gd)
		}
		return base.MakeOutputError(404, "Graph not found")
	}
	return base.MakeObjectOutput(gd)
}

// ExecuteGraphFromStore executes a graph after loading it from the store
func (m *OpGraphBuilder) ExecuteGraphFromStore(ctx *base.Context, input *base.OperatorIO, args GetGraphArgs) *base.OperatorIO {
	gd, err := freepsstore.GetGraph(args.GraphName)
	if err != nil {
		return base.MakeOutputError(404, "Graph not found in store: %v", err)
	}
	return m.GE.ExecuteAdHocGraph(ctx, "ExecuteFromStore/"+args.GraphName, gd, make(map[string]string), input)
}
