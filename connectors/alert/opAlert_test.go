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
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestOpAlert(t *testing.T) {
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")

	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	ge := freepsflow.NewFlowEngine(ctx, cr, func() {})
	op := OpAlert{CR: cr, GE: ge}
	ops := base.MakeFreepsOperators(&op, cr, ctx)
	opA := ops[0]

	sug := opA.GetArgSuggestions("SetAlert", "ExpiresInDuration", base.MakeEmptyFunctionArguments())
	assert.Assert(t, sug != nil)
	_, ok := sug["2s"]
	assert.Assert(t, ok)

	// SetAlert: not enough arguments
	res := opA.Execute(ctx, "SetAlert", base.NewSingleFunctionArgument("Name", "test_alert"), base.MakeEmptyOutput())
	assert.Assert(t, res.IsError())

	// SetAlert: invalid entry in the internal namespace
	ns, err := freepsstore.GetGlobalStore().GetNamespace("_alerts")
	assert.NilError(t, err)
	entry := ns.SetValue("test.test_alert_invalid", base.MakePlainOutput("invalid"), ctx)
	assert.Assert(t, !entry.IsError())

	res = op.SetAlert(ctx, base.MakeEmptyOutput(), Alert{Name: "test_alert_invalid", Category: "test", Severity: 2}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, res.IsError())

	// GetActiveAlerts: no alerts, skips the invalid entry
	res = op.GetActiveAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.GetString() == "{}")

	// get rid of the invalid entry
	ns.DeleteValue("test.test_alert_invalid")

	res = op.SetAlert(ctx, base.MakeEmptyOutput(), Alert{Name: "foo", Severity: 2, Category: "test"}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, !res.IsError())
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "foo", Category: "test"})
	assert.Assert(t, res.HTTPCode == http.StatusOK)
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "bar", Category: "test"})
	assert.Assert(t, res.HTTPCode == http.StatusNotFound)
	res = op.GetActiveAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, !res.IsError() && len(res.GetString()) > 5)
	res = op.HasAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.HTTPCode == http.StatusOK)
	res = op.SilenceAlert(ctx, base.MakeEmptyOutput(), SilenceAlertArgs{Name: "foo", SilenceDuration: time.Minute, Category: "test"})
	assert.Assert(t, !res.IsError())
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "foo", Category: "test"})
	assert.Assert(t, res.HTTPCode == http.StatusExpectationFailed)
	trueVal := true
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "foo", Category: "test", IgnoreSilence: &trueVal})
	assert.Assert(t, res.HTTPCode == http.StatusOK)
	res = op.ResetSilence(ctx, base.MakeEmptyOutput(), ResetAlertArgs{Name: "foo", Category: "test"})
	assert.Assert(t, !res.IsError())
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "foo", Category: "test"})
	assert.Assert(t, res.HTTPCode == http.StatusOK)
	res = op.ResetAlert(ctx, base.MakeEmptyOutput(), ResetAlertArgs{Name: "foo", Category: "test"})
	assert.Assert(t, !res.IsError())
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "foo", Category: "test"})
	assert.Assert(t, res.HTTPCode == http.StatusExpectationFailed)
	res = op.GetShortAlertString(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.IsEmpty())
	res = op.HasAlerts(ctx, base.MakeEmptyOutput(), GetAlertArgs{})
	assert.Assert(t, res.HTTPCode == http.StatusExpectationFailed)

	// TestActiveDuration
	expiredDuration := 10 * time.Second
	res = op.SetAlert(ctx, base.MakeEmptyOutput(), Alert{Name: "oldAlert", Severity: 2, Category: "test", ExpiresInDuration: &expiredDuration}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, !res.IsError())
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "oldAlert", Category: "test"})
	assert.Assert(t, res.HTTPCode == http.StatusOK)
	alertDuration := 100 * time.Millisecond
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "oldAlert", Category: "test", ActiveDuration: &alertDuration})
	assert.Assert(t, res.HTTPCode == http.StatusExpectationFailed) // alert is not active for long enough
	time.Sleep(alertDuration)
	res = op.IsActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "oldAlert", Category: "test", ActiveDuration: &alertDuration})
	assert.Assert(t, res.HTTPCode == http.StatusOK) // alert is active for long enough
	// set alert again
	res = op.SetAlert(ctx, base.MakeEmptyOutput(), Alert{Name: "oldAlert", Severity: 2, Category: "test"}, base.MakeEmptyFunctionArguments())
	assert.Assert(t, !res.IsError())
	res = op.GetActiveAlert(ctx, base.MakeEmptyOutput(), IsActiveAlertArgs{Name: "oldAlert", Category: "test", ActiveDuration: &alertDuration})
	assert.Assert(t, res.HTTPCode == http.StatusOK) // a second set doesn't change the duration
	// make sure, GetActiveAlerts returns the expected alert
	var alert ReadableAlert
	err = res.ParseJSON(&alert)
	assert.NilError(t, err)
	assert.Assert(t, alert.Counter == 2)
	assert.Assert(t, alert.Name == "oldAlert")
	assert.Assert(t, alert.Category == "test")
	assert.Assert(t, alert.Severity == 2)
	assert.Assert(t, alert.ExpiresInDuration < expiredDuration)
}

func createTestFlow(keyToSet string) freepsflow.FlowDesc {
	gd := freepsflow.FlowDesc{Operations: []freepsflow.FlowOperationDesc{{Operator: "utils", Function: "echoArguments"}, {Operator: "store", Function: "set", InputFrom: "#0", Arguments: map[string]string{"namespace": "test", "key": keyToSet}}}}
	return gd
}

func TestTriggers(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")
	assert.NilError(t, err)
	ge := freepsflow.NewFlowEngine(ctx, cr, func() {})
	op := &OpAlert{CR: cr, GE: ge}
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge},
		&freepsutils.OpUtils{},
		op,
	}

	for _, op := range availableOperators {
		ge.AddOperators(base.MakeFreepsOperators(op, cr, ctx))
	}

	err = ge.AddFlow(ctx, "testflowSev2", createTestFlow("testflowSev2"), false)
	err = ge.AddFlow(ctx, "testflowSev3", createTestFlow("testflowSev3"), false)
	err = ge.AddFlow(ctx, "testflowOnSet", createTestFlow("testflowOnSet"), false)
	err = ge.AddFlow(ctx, "testflowOnReset", createTestFlow("testflowOnReset"), false)
	assert.NilError(t, err)

	out := op.SetSeverityTrigger(ctx, base.MakeEmptyOutput(), SeverityTrigger{Severity: 2, FlowID: "testflowSev2"})
	assert.Assert(t, !out.IsError())

	out = op.SetAlertSetTrigger(ctx, base.MakeEmptyOutput(), NameTrigger{Name: "testcategory.testalert", FlowID: "testflowOnSet"})
	assert.Assert(t, !out.IsError())

	out = op.SetAlertResetTrigger(ctx, base.MakeEmptyOutput(), NameTrigger{Name: "testcategory.testalert", FlowID: "testflowOnReset"})
	assert.Assert(t, !out.IsError())

	out = op.SetAlertResetTrigger(ctx, base.MakeEmptyOutput(), NameTrigger{Name: "testcategory.testalert2", FlowID: "testflowOnReset"})
	assert.Assert(t, !out.IsError())

	/* Test the triggers when alert is acitvated*/
	dur := time.Minute
	ge.SetSystemAlert(ctx, "testalert", "testcategory", 2, fmt.Errorf("opsi"), &dur)

	ns, err := freepsstore.GetGlobalStore().GetNamespace("test")
	assert.NilError(t, err)
	assert.Assert(t, ns.GetValue("testflowSev2") != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue("testflowSev3") == freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue("testflowOnSet") != freepsstore.NotFoundEntry)
	assert.Assert(t, ns.GetValue("testflowOnReset") == freepsstore.NotFoundEntry)

	i := ns.DeleteOlder(time.Duration(0))
	assert.Assert(t, i == 2)

	/* Test the reset triggers */

	// the alert is active, so it should trigger now
	ge.ResetSystemAlert(ctx, "testalert", "testcategory")
	assert.Assert(t, ns.GetValue("testflowOnReset") != freepsstore.NotFoundEntry)

	i = ns.DeleteOlder(time.Duration(0))
	assert.Assert(t, i == 1)

	// the alert is inactive, so it should not trigger again
	ge.ResetSystemAlert(ctx, "testalert", "testcategory")
	assert.Assert(t, ns.GetValue("testflowOnReset") == freepsstore.NotFoundEntry)

	// the alert is inactive but new, so freeps might have restarted, trigger again
	ge.ResetSystemAlert(ctx, "testalert2", "testcategory")
	assert.Assert(t, ns.GetValue("testflowOnReset") != freepsstore.NotFoundEntry)

	i = ns.DeleteOlder(time.Duration(0))
	assert.Assert(t, i == 1)

	op.SilenceAlert(ctx, base.MakeEmptyOutput(), SilenceAlertArgs{Name: "testalert", Category: "testcategory", SilenceDuration: time.Minute})
	ge.SetSystemAlert(ctx, "testalert", "testcategory", 2, fmt.Errorf("opsi"), &dur)
	ge.ResetSystemAlert(ctx, "testalert", "testcategory")

	// no flow should have been executed:
	i = ns.DeleteOlder(time.Duration(0))
	assert.Assert(t, i == 0)
}
