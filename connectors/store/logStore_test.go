package freepsstore

import (
	"sync"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"gotest.tools/v3/assert"
)

func TestLogExpiration(t *testing.T) {
	nsStore := logStoreNamespace{entries: []StoreEntry{}, offset: 0, nsLock: sync.Mutex{}}
	nsStore.SetValue("", base.MakePlainOutput("1"), "testing")
	e := nsStore.GetValue("1")
	assert.Equal(t, e, NotFoundEntry)

	e = nsStore.GetValue("0")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "1")
}
