package opalert

import (
	"net/http"
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestOpAlert(t *testing.T) {
	ctx := base.NewContext(logrus.StandardLogger())

	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	op := OpAlert{CR: cr, GE: nil}
	ops := base.MakeFreepsOperators(&op, cr, ctx)
	opA := ops[0]

	sug := opA.GetArgSuggestions("SetAlert", "ExpiresInDuration", map[string]string{})
	assert.Assert(t, sug != nil)
	_, ok := sug["2s"]
	assert.Assert(t, ok)
	res := opA.Execute(ctx, "SetAlert", map[string]string{"Name": "test_alert"}, base.MakeEmptyOutput())
	assert.Assert(t, res.IsError())

	res = op.GetActiveAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.GetString() == "[]")
	res = op.SetAlert(ctx, base.MakeEmptyOutput(), Alert{Name: "foo", Severity: 2})
	assert.Assert(t, !res.IsError())
	res = op.GetActiveAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, !res.IsError() && len(res.GetString()) > 5)
	res = op.ResetAlert(ctx, base.MakeEmptyOutput(), ResetAlertArgs{Name: "foo"})
	res = op.GetShortAlertString(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.IsEmpty())
	res = op.HasAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.HTTPCode == http.StatusExpectationFailed)
}