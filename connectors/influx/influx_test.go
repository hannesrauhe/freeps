//go:build !noinflux

package influx

import (
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
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

	op := OperatorInflux{CR: cr, GE: ge}
	cfg := op.GetDefaultConfig()
	op.InitCopyOfOperator(ctx, cfg, "influx")
	assert.Equal(t, cfg.(*InfluxConfig).Enabled, false)

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
	migratedConfig := cfg.(*InfluxConfig)
	assert.Equal(t, migratedConfig.Bucket, "test-bucket-1")
	assert.Equal(t, migratedConfig.Org, "test-org-1")
	assert.Equal(t, migratedConfig.Token, "test-token-1")
	assert.Equal(t, migratedConfig.URL, "https://example.com")
	assert.Equal(t, migratedConfig.Enabled, true)
	cr.WriteSection("influx", cfg, true)

	old := &OldFreepsFluxConfig{}
	err = cr.ReadSectionWithDefaults("freepsflux", old)
	assert.NilError(t, err)
	assert.Equal(t, len(old.InfluxdbConnections), 1)

	// the second connection should be migrated to a new config
	cfg2 := op.GetDefaultConfig()
	op.InitCopyOfOperator(ctx, cfg2, "influx.second")
	migratedConfig2 := cfg2.(*InfluxConfig)
	assert.Equal(t, migratedConfig2.Bucket, "test-bucket-2")
	assert.Equal(t, migratedConfig2.Org, "test-org-2")
	assert.Equal(t, migratedConfig2.Token, "test-token-2")
	assert.Equal(t, migratedConfig2.URL, "http://example.org")
	assert.Equal(t, migratedConfig2.Enabled, true)

	old = &OldFreepsFluxConfig{}
	err = cr.ReadSectionWithDefaults("freepsflux", old)
	assert.NilError(t, err)
	assert.Equal(t, len(old.InfluxdbConnections), 0)
}
