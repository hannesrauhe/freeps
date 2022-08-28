package freepsgraph

import (
	"testing"

	"gotest.tools/v3/assert"
)

type MockOperator struct {
	DoCount      int
	LastFunction string
	LastJSON     []byte
}

func (*MockOperator) Execute(fn string, mainArgs map[string]string, mainInput *OperatorIO) *OperatorIO {
	return mainInput
}

var _ FreepsOperator = &MockOperator{}

func TestCallFreepsOperator(t *testing.T) {
	g := NewGraph(&GraphDesc{Name: "Test", Operations: make([]GraphOperationDesc, 0)})
	o := g.ExecuteOperation("", &GraphOperationDesc{Name: "myname", Operator: "mock", Arguments: make(map[string]string), InputFrom: "NOTTHERE", ArgumentsFrom: "NOTTHERE"}, make(map[string]string))
	assert.Assert(t, o.IsError())
}
