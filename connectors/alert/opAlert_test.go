package opalert

import (
	"fmt"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	freepsutils "github.com/hannesrauhe/freeps/connectors/utils"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestOpAlert(t *testing.T) {
	ctx := base.NewContext(logrus.StandardLogger())

	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	ge := freepsgraph.NewGraphEngine(ctx, cr, func() {})
	op := OpAlert{CR: cr, GE: ge}
	ops := base.MakeFreepsOperators(&op, cr, ctx)
	opA := ops[0]

	sug := opA.GetArgSuggestions("SetAlert", "ExpiresInDuration", map[string]string{})
	assert.Assert(t, sug != nil)
	_, ok := sug["2s"]
	assert.Assert(t, ok)
	res := opA.Execute(ctx, "SetAlert", base.NewSingleFunctionArgument("Name", "test_alert"), base.MakeEmptyOutput())
	assert.Assert(t, res.IsError())

	res = op.GetActiveAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.GetString() == "[]")
	res = op.SetAlert(ctx, base.MakeEmptyOutput(), Alert{Name: "foo", Severity: 2}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, !res.IsError())
	res = op.GetActiveAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, !res.IsError() && len(res.GetString()) > 5)
	res = op.ResetAlert(ctx, base.MakeEmptyOutput(), ResetAlertArgs{Name: "foo"})
	res = op.GetShortAlertString(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.IsEmpty())
	res = op.HasAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.HTTPCode == http.StatusExpectationFailed)
}

func createTestGraph(keyToSet string) freepsgraph.GraphDesc {
	gd := freepsgraph.GraphDesc{Operations: []freepsgraph.GraphOperationDesc{{Operator: "utils", Function: "echoArguments"}, {Operator: "store", Function: "set", InputFrom: "#0", Arguments: map[string]string{"namespace": "test", "key": keyToSet}}}}
	return gd
}

func TestTriggers(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	ctx := base.NewContext(logrus.StandardLogger())
	assert.NilError(t, err)
	ge := freepsgraph.NewGraphEngine(ctx, cr, func() {})
	op := &OpAlert{CR: cr, GE: ge}
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge},
		&freepsutils.OpUtils{},
		op,
	}

	for _, op := range availableOperators {
		ge.AddOperators(base.MakeFreepsOperators(op, cr, ctx))
	}

	err = ge.AddGraph(ctx, "testgraphSev2", createTestGraph("testgraphSev2"), false)
	err = ge.AddGraph(ctx, "testgraphSev3", createTestGraph("testgraphSev3"), false)
	assert.NilError(t, err)

	out := op.SetSeverityTrigger(ctx, base.MakeEmptyOutput(), SeverityTrigger{Severity: 2, GraphID: "testgraphSev2"})
	assert.Assert(t, !out.IsError())

	dur := time.Minute
	ge.SetSystemAlert(ctx, "testalert", "testcategory", 2, fmt.Errorf("opsi"), &dur)
	assert.Assert(t, !out.IsError())

	ns, err := freepsstore.GetGlobalStore().GetNamespace("test")
	assert.NilError(t, err)
	assert.Assert(t, ns.GetValue("testgraphSev2") != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue("testgraphSev3") == freepsstore.NotFoundEntry)
}
