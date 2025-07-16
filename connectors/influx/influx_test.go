//go:build !noinflux

package influx_test

import (
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/connectors/influx"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsd/helper"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestMigrateOldConfig(t *testing.T) {
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")
	ge := freepsflow.NewFlowEngine(ctx, cr, func() {})

	op := influx.OperatorInflux{CR: cr, GE: ge}
	cfg := op.GetDefaultConfig()
	op.InitCopyOfOperator(ctx, cfg, "influx")
	assert.Equal(t, cfg.(*influx.InfluxConfig).Enabled, true)

	cr.WriteSectionBytes("freepsflux", []byte(`{
		"Enabled": true,
		"IgnoreNotPresent": true,
		"InfluxdbConnections": [
		  {
			"Bucket": "test-bucket-1",
			"Org": "test-org-1",
			"Token": "test-token-1",
			"URL": "https://example.com"
		  },
		  {
			"Bucket": "test-bucket-2",
			"Org": "test-org-2",
			"Token": "test-token-2",
			"URL": "http://example.org"
		  }
		],
		"Namespace": "_influx"
		}`))
	cr.WriteBackConfigIfChanged()

	cfg = op.GetDefaultConfig()
	// the section should automatically be renamed from freepsflux to influx
	err = cr.ReadSectionWithDefaults("influx", cfg)
	assert.NilError(t, err)

	op.InitCopyOfOperator(ctx, cfg, "influx")
	migratedConfig := cfg.(*influx.InfluxConfig)
	assert.Equal(t, migratedConfig.Bucket, "test-bucket-1")
	assert.Equal(t, migratedConfig.Org, "test-org-1")
	assert.Equal(t, migratedConfig.Token, "test-token-1")
	assert.Equal(t, migratedConfig.URL, "https://example.com")
	assert.Equal(t, migratedConfig.Enabled, true)
	cr.WriteSection("influx", cfg, true)

	old := &influx.OldFreepsFluxConfig{}
	err = cr.ReadSectionWithDefaults("freepsflux", old)
	assert.NilError(t, err)
	assert.Equal(t, len(old.InfluxdbConnections), 1)

	// the second connection should be migrated to a new config
	cfg2 := op.GetDefaultConfig()
	op.InitCopyOfOperator(ctx, cfg2, "influx.second")
	migratedConfig2 := cfg2.(*influx.InfluxConfig)
	assert.Equal(t, migratedConfig2.Bucket, "test-bucket-2")
	assert.Equal(t, migratedConfig2.Org, "test-org-2")
	assert.Equal(t, migratedConfig2.Token, "test-token-2")
	assert.Equal(t, migratedConfig2.URL, "http://example.org")
	assert.Equal(t, migratedConfig2.Enabled, true)

	old = &influx.OldFreepsFluxConfig{}
	err = cr.ReadSectionWithDefaults("freepsflux", old)
	assert.NilError(t, err)
	assert.Equal(t, len(old.InfluxdbConnections), 0)
}

func TestPushFieldsInternal(t *testing.T) {
	ctx, ge, cr := helper.SetupEngineWithCommonOperators(t, map[string]interface{}{
		"influx": &influx.InfluxConfig{
			Enabled:        true,
			StoreNamespace: "_influx",
		}})

	ge.AddOperators(base.MakeFreepsOperators(&influx.OperatorInflux{CR: cr, GE: ge}, cr, ctx))

	ge.StartListening(ctx)

	op := influx.GetGlobalInfluxInstance("default")
	assert.Assert(t, op != nil)
	st := freepsstore.GetGlobalStore()
	assert.Assert(t, st != nil)

	// Test with empty fields
	result := op.PushFieldsInternal("measurement", nil, nil, ctx)
	assert.Equal(t, result.IsEmpty(), true)

	// Test with non-empty fields
	result = op.PushFieldsInternal("measurement", nil, map[string]interface{}{"field1": 1, "field2": "2"}, ctx)
	assert.Equal(t, result.IsEmpty(), true)

	ns, err := st.GetNamespace("_influx")
	assert.NilError(t, err)
	vs := ns.GetAllValues(10)
	assert.Equal(t, len(vs), 1)
	for _, v := range vs {
		linebuffer := v.GetString()
		assert.Assert(t, utils.StringStartsWith(linebuffer, "measurement, field1=1i,field2=\"2\""), "Line protocol looks unexpected: %v", linebuffer)
	}
}
