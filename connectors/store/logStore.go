package freepsstore

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

type logStoreNamespace struct {
	entries []StoreEntry
	offset  int
	nsLock  sync.Mutex
}

var _ StoreNamespace = &logStoreNamespace{}

func (s *logStoreNamespace) setValueUnlocked(keyStr string, newValue *base.OperatorIO, modifiedBy string) StoreEntry {
	if keyStr == "" {
		x := StoreEntry{newValue, time.Now(), modifiedBy}
		s.entries = append(s.entries, x)
		return x
	}
	keyNoOffset, err := strconv.Atoi(keyStr)
	if err != nil {
		return MakeEntryError(http.StatusBadRequest, "%v is not a valid key", keyStr)
	}
	key := keyNoOffset - s.offset
	if key < 0 || key >= len(s.entries) {
		return NotFoundEntry
	}
	s.entries[key].data = newValue
	s.entries[key].modifiedBy = modifiedBy
	return s.entries[key]
}

func (s *logStoreNamespace) getValueUnlocked(keyStr string) (int, StoreEntry) {
	keyNoOffset, err := strconv.Atoi(keyStr)
	if err != nil {
		return -1, MakeEntryError(http.StatusBadRequest, "%v is not a valid key", keyStr)
	}
	key := keyNoOffset - s.offset
	if key < 0 || key >= len(s.entries) {
		return -1, NotFoundEntry
	}
	return key, s.entries[key]
}

// GetValue from the StoreNamespace
func (s *logStoreNamespace) GetValue(keyStr string) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	_, e := s.getValueUnlocked(keyStr)
	return e
}

// GetValueBeforeExpiration gets the value from the StoreNamespace, but returns error if older than maxAge
func (s *logStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) StoreEntry {
	e := s.GetValue(key)
	if e == NotFoundEntry {
		return e
	}
	ts := e.timestamp
	if ts.Add(maxAge).Before(time.Now()) {
		e.data = base.MakeOutputError(http.StatusGone, "Entry is older than %v", maxAge)
		return e
	}
	return e
}

// SetValue in the StoreNamespace
func (s *logStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy string) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	return s.setValueUnlocked(key, io, modifiedBy)
}

// SetAll sets all values in the StoreNamespace
func (s *logStoreNamespace) SetAll(valueMap map[string]interface{}, modifiedBy string) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	for k, v := range valueMap {
		s.setValueUnlocked(k, base.MakeObjectOutput(v), modifiedBy)
	}
	return base.MakeEmptyOutput()
}

// CompareAndSwap sets the value if the string representation of the already stored value is as expected
func (s *logStoreNamespace) CompareAndSwap(keyStr string, expected string, newValue *base.OperatorIO, modifiedBy string) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	key, oldV := s.getValueUnlocked(keyStr)
	if oldV.IsError() {
		return oldV
	}
	if oldV.data == nil || oldV.data.GetString() != expected {
		return MakeEntryError(http.StatusConflict, "old value is different from expectation")
	}
	s.entries[key].data = newValue
	s.entries[key].modifiedBy = modifiedBy
	return s.entries[key]
}

// UpdateTransaction updates the value in the StoreNamespace by calling the function fn with the current value
func (s *logStoreNamespace) UpdateTransaction(keyStr string, fn func(*base.OperatorIO) *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()

	var oldV *base.OperatorIO
	_, oldEntry := s.getValueUnlocked(keyStr)
	if oldEntry == NotFoundEntry || oldEntry.data == nil {
		oldV = base.MakeEmptyOutput()
	} else if oldEntry.IsError() {
		return oldEntry.data
	} else {
		oldV = oldEntry.data
	}

	out := fn(oldV)
	if out.IsError() {
		return out
	}
	return s.setValueUnlocked(keyStr, out, modifiedBy).GetData()
}

// OverwriteValueIfOlder sets the value only if the key does not exist or has been written before maxAge
func (s *logStoreNamespace) OverwriteValueIfOlder(keyStr string, newValue *base.OperatorIO, maxAge time.Duration, modifiedBy string) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	n := time.Now()
	key, oldEntry := s.getValueUnlocked(keyStr)

	if oldEntry == NotFoundEntry {
		return s.setValueUnlocked(keyStr, newValue, modifiedBy)
	}

	if oldEntry.IsError() {
		return oldEntry
	}

	if oldEntry.timestamp.Add(maxAge).After(n) {
		return MakeEntryError(http.StatusConflict, "%v already exists and is only %v old", keyStr, n.Sub(oldEntry.timestamp))
	}

	s.entries[key].data = newValue
	s.entries[key].modifiedBy = modifiedBy
	return s.entries[key]
}

// DeleteValue from the StoreNamespace
func (s *logStoreNamespace) DeleteValue(key string) {
	panic("not implemented")
}

func (s *logStoreNamespace) getKeyString(key int) string {
	return fmt.Sprintf("%v", key+s.offset)
}

// GetKeys returns all keys in the StoreNamespace
func (s *logStoreNamespace) GetKeys() []string {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	keys := []string{}
	for k := range s.entries {
		keys = append(keys, s.getKeyString(k))
	}
	return keys
}

// Len returns the number of entries in the StoreNamespace
func (s *logStoreNamespace) Len() int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	return len(s.entries)
}

// GetAllValues from the StoreNamespace
func (s *logStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	copy := map[string]*base.OperatorIO{}
	counter := 0
	for k, v := range s.entries {
		copy[s.getKeyString(k)] = v.data
		counter++
		if limit != 0 && counter >= limit {
			return copy
		}
	}
	return copy
}

// GetSearchResultWithMetadata searches through all keys, optionally finds substring in key, value and ID, and returns only records younger than maxAge
func (s *logStoreNamespace) GetSearchResultWithMetadata(keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration) map[string]StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	copy := map[string]StoreEntry{}
	for k, v := range s.entries {
		if matches(s.getKeyString(k), v, keyPattern, valuePattern, modifiedByPattern, minAge, maxAge, tnow) {
			copy[s.getKeyString(k)] = v
		}
	}
	return copy
}

// DeleteOlder deletes records older than maxAge
func (s *logStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	timeCut := 0
	for k, md := range s.entries {
		ts := md.timestamp
		if ts.Add(maxAge).Before(tnow) {
			timeCut = k
		} else {
			break
		}
	}
	s.entries = s.entries[timeCut:]
	s.offset += timeCut
	return timeCut
}

// Trim deletes all but the top k records sorted by timestamp
func (s *logStoreNamespace) Trim(k int) int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()

	if k >= len(s.entries) {
		return 0
	}
	timeCut := len(s.entries) - k
	s.entries = s.entries[timeCut:]
	s.offset += timeCut
	return timeCut
}
