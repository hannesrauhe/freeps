package helper

import (
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/sensor"
	freepsutils "github.com/hannesrauhe/freeps/connectors/utils"

	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"

	opalert "github.com/hannesrauhe/freeps/connectors/alert"
	freepsmetrics "github.com/hannesrauhe/freeps/connectors/metrics"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
)

func SetupEngineWithCommonOperators(t *testing.T, configSections map[string]interface{}) (*base.Context, *freepsflow.FlowEngine, *utils.ConfigReader) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")
	ge := freepsflow.NewFlowEngine(ctx, cr, func() {})

	if configSections != nil {
		for sectionName, configSection := range configSections {
			cr.WriteSection(sectionName, configSection, false)
		}
	}
	availableOperators := []base.FreepsOperator{
		&freepsstore.OpStore{CR: cr, GE: ge}, // must be first so that other operators can use the store
		&opalert.OpAlert{CR: cr, GE: ge},     // must be second so that other operators can use alerts
		&sensor.OpSensor{CR: cr, GE: ge},     // must be third so that other operators can use sensors
		&freepsmetrics.OpMetrics{CR: cr, GE: ge},
		&freepsutils.OpUtils{},
	}

	for _, op := range availableOperators {
		ge.AddOperators(base.MakeFreepsOperators(op, cr, ctx))
	}
	return ctx, ge, cr
}
