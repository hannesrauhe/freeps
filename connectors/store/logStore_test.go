package freepsstore

import (
	"fmt"
	"sync"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestLogExpiration(t *testing.T) {
	nsStore := logStoreNamespace{entries: []StoreEntry{}, offset: 0, nsLock: sync.Mutex{}}

	i := 0
	for i < 10 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), base.NewBaseContextWithReason(logrus.StandardLogger(), fmt.Sprintf("modified-%d", i)))
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
	assert.Equal(t, e.GetReason(), "modified-5")
	ts := e.GetTimestamp()

	e = nsStore.SetValue("5", base.MakePlainOutput("new-5"), base.NewBaseContextWithReason(logrus.StandardLogger(), "modified-later"))
	assert.Assert(t, !e.IsError())
	e = nsStore.GetValue("5")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "new-5")
	assert.Equal(t, e.GetReason(), "modified-later")
	assert.Equal(t, e.GetTimestamp(), ts)

	e = nsStore.GetValue("x")
	assert.Assert(t, e.IsError())

	s := nsStore.GetKeys()
	assert.Equal(t, len(s), 5)
	assert.Equal(t, s[0], "5")
	assert.Equal(t, s[4], "9")
	for i < 30 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), base.NewBaseContextWithReason(logrus.StandardLogger(), fmt.Sprintf("modified-%d", i)))
		i += 1
	}

	s = nsStore.GetKeys()
	assert.Equal(t, nsStore.Len(), 25)
	assert.Equal(t, len(s), 25)
	assert.Equal(t, s[0], "05")
	nsStore.Trim(20)
	assert.Equal(t, nsStore.Len(), 20)
	s = nsStore.GetKeys()
	assert.Equal(t, s[0], "10")

	for i < 101 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), base.NewBaseContextWithReason(logrus.StandardLogger(), fmt.Sprintf("modified-%d", i)))
		i += 1
	}

	nsStore.Trim(10)
	s = nsStore.GetKeys()
	assert.Equal(t, s[0], "091")
	nsStore.Trim(0)
	s = nsStore.GetKeys()
	assert.Equal(t, len(s), 0)

	for i < 102 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), base.NewBaseContextWithReason(logrus.StandardLogger(), fmt.Sprintf("modified-%d", i)))
		i += 1
	}
	vm := nsStore.GetAllValues(100)
	assert.Equal(t, vm["101"].GetString(), "101")
}

func TestAutoTrim(t *testing.T) {
	nsStore := logStoreNamespace{entries: []StoreEntry{}, offset: 0, nsLock: sync.Mutex{}, AutoTrim: 100}

	i := 0
	for i < 100 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), base.NewBaseContextWithReason(logrus.StandardLogger(), fmt.Sprintf("modified-%d", i)))
		i += 1
	}

	e := nsStore.GetValue("5")
	assert.Assert(t, !e.IsError())
	assert.Equal(t, e.GetData().GetString(), "5")

	assert.Equal(t, nsStore.Len(), 100)

	for i < 130 {
		nsStore.SetValue("", base.MakePlainOutput(fmt.Sprintf("%d", i)), base.NewBaseContextWithReason(logrus.StandardLogger(), fmt.Sprintf("modified-%d", i)))
		i += 1
		assert.Assert(t, nsStore.Len() < 110)
	}

	assert.Equal(t, nsStore.Len(), 100)
}
