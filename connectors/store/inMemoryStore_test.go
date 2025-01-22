package freepsstore

import (
	"math"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestCaseInsensitiveBehaviour(t *testing.T) {
	nsStore := newInMemoryStoreNamespace()
	nsStore.SetValue("Test", base.MakePlainOutput("value"), base.NewBaseContextWithReason(logrus.StandardLogger(), "test"))
	e := nsStore.GetValue("test")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "value")
	e = nsStore.GetValue("TEST")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "value")

	keys := nsStore.GetKeys()
	assert.Equal(t, len(keys), 1)
	assert.Equal(t, keys[0], "Test")

	allValues := nsStore.GetSearchResultWithMetadata("tEst", "", "", 0, math.MaxInt64)
	assert.Equal(t, len(allValues), 1)
	assert.Equal(t, allValues["Test"].GetData().GetString(), "value")

	nsStore.DeleteValue("TeSt")
	e = nsStore.GetValue("test")
	assert.Equal(t, e, NotFoundEntry)

	keys = nsStore.GetKeys()
	assert.Equal(t, len(keys), 0)
}
