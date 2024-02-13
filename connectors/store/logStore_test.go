package freepsstore

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"gotest.tools/v3/assert"
)

func TestLogExpiration(t *testing.T) {
	nsStore := logStoreNamespace{entries: []StoreEntry{}, offset: 0, nsLock: sync.Mutex{}}

	i := 0
	for i < 10 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), fmt.Sprintf("modified-%d", i))
		i += 1
	}

	e := nsStore.GetValue("10")
	assert.Equal(t, e, NotFoundEntry)

	e = nsStore.GetValue("5")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "5")

	deleted := nsStore.Trim(5)
	assert.Equal(t, deleted, 5)

	e = nsStore.GetValue("1")
	assert.Equal(t, e, NotFoundEntry)

	e = nsStore.GetValue("4")
	assert.Equal(t, e, NotFoundEntry)

	e = nsStore.GetValue("5")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "5")
	assert.Equal(t, e.GetModifiedBy(), "modified-5")
	ts := e.GetTimestamp()

	e = nsStore.SetValue("5", base.MakePlainOutput("new-5"), "modified-later")
	assert.Assert(t, !e.IsError())
	e = nsStore.GetValue("5")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "new-5")
	assert.Equal(t, e.GetModifiedBy(), "modified-later")
	assert.Equal(t, e.GetTimestamp(), ts)

	e = nsStore.GetValue("x")
	assert.Assert(t, e.IsError())

	s := nsStore.GetKeys()
	assert.Equal(t, s[0], "5")
	assert.Equal(t, len(s), 5)
	for i < 30 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), fmt.Sprintf("modified-%d", i))
		i += 1
	}

	s = nsStore.GetKeys()
	assert.Equal(t, nsStore.Len(), 25)
	assert.Equal(t, len(s), 25)
	assert.Equal(t, s[0], "05")
}
