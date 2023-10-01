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

// AddGraphArgs are the arguments for the GraphBuilder function
type AddGraphArgs struct {
	GraphName string
	Overwrite bool
}

// AddGraph adds a graph to the graph engine
func (m *OpGraphBuilder) AddGraph(ctx *base.Context, input *base.OperatorIO, args AddGraphArgs) *base.OperatorIO {
	gd := freepsgraph.GraphDesc{}
	err := input.ParseJSON(&gd)
	if err != nil {
		return base.MakeOutputError(400, "Invalid graph: %v", err)
	}
	err = m.GE.AddGraph(args.GraphName, gd, args.Overwrite)
	if err != nil {
		return base.MakeOutputError(400, "Could not add graph: %v", err)
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
	FromStore bool
}

// GetGraph returns a graph from the graph engine
func (m *OpGraphBuilder) GetGraph(ctx *base.Context, input *base.OperatorIO, args GetGraphArgs) *base.OperatorIO {
	gd, ok := m.GE.GetGraphDesc(args.GraphName)
	if !ok {
		if args.FromStore {
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
