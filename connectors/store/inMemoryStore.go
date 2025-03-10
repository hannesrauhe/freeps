package freepsstore

import (
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type inMemoryStoreNamespace struct {
	entries          map[string]StoreEntry
	originalCaseKeys map[string]string
	nsLock           sync.Mutex
}

var _ StoreNamespace = &inMemoryStoreNamespace{}

func newInMemoryStoreNamespace() *inMemoryStoreNamespace {
	return &inMemoryStoreNamespace{
		entries:          map[string]StoreEntry{},
		originalCaseKeys: map[string]string{},
	}
}

func (s *inMemoryStoreNamespace) getValueUnlocked(key string) StoreEntry {
	lowerCaseKey := strings.ToLower(key)
	e, ok := s.entries[lowerCaseKey]
	if !ok {
		return NotFoundEntry
	}
	return e
}

func (s *inMemoryStoreNamespace) setValueUnlocked(key string, newValue *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	lowerCaseKey := strings.ToLower(key)
	x := StoreEntry{newValue, time.Now(), modifiedBy}
	s.entries[lowerCaseKey] = x
	s.originalCaseKeys[lowerCaseKey] = key
	return x
}

func (s *inMemoryStoreNamespace) deleteValueUnlocked(key string) {
	lowerCaseKey := strings.ToLower(key)
	delete(s.entries, lowerCaseKey)
	delete(s.originalCaseKeys, lowerCaseKey)
}

// GetValue from the StoreNamespace
func (s *inMemoryStoreNamespace) GetValue(key string) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	return s.getValueUnlocked(key)
}

// GetValueBeforeExpiration gets the value from the StoreNamespace, but returns error if older than maxAge
func (s *inMemoryStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) StoreEntry {
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
func (s *inMemoryStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	return s.setValueUnlocked(key, io, modifiedBy)
}

// SetAll sets all values in the StoreNamespace
func (s *inMemoryStoreNamespace) SetAll(valueMap map[string]interface{}, modifiedBy *base.Context) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	for k, v := range valueMap {
		s.setValueUnlocked(k, base.MakeObjectOutput(v), modifiedBy)
	}
	return base.MakeEmptyOutput()
}

// CompareAndSwap sets the value if the string representation of the already stored value is as expected
func (s *inMemoryStoreNamespace) CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	oldV := s.getValueUnlocked(key)
	if oldV == NotFoundEntry {
		return NotFoundEntry
	}
	if oldV.data == nil || oldV.data.GetString() != expected {
		return MakeEntryError(http.StatusConflict, "old value is different from expectation")
	}
	return s.setValueUnlocked(key, newValue, modifiedBy)
}

// UpdateTransaction updates the value in the StoreNamespace by calling the function fn with the current value
func (s *inMemoryStoreNamespace) UpdateTransaction(key string, fn func(StoreEntry) *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	oldEntry := s.getValueUnlocked(key)

	out := fn(oldEntry)
	if out.IsError() {
		return MakeEntry(out, modifiedBy)
	}
	if out.HTTPCode == http.StatusContinue {
		return oldEntry
	}
	return s.setValueUnlocked(key, out, modifiedBy)
}

// OverwriteValueIfOlder sets the value only if the key does not exist or has been written before maxAge
func (s *inMemoryStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy *base.Context) StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	n := time.Now()
	md := s.getValueUnlocked(key)
	if md != NotFoundEntry && md.timestamp.Add(maxAge).After(n) {
		return MakeEntryError(http.StatusConflict, "%v already exists and is only %v old", key, n.Sub(md.timestamp))
	}
	return s.setValueUnlocked(key, io, modifiedBy)
}

// DeleteValue from the StoreNamespace
func (s *inMemoryStoreNamespace) DeleteValue(key string) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	s.deleteValueUnlocked(key)
}

// GetKeys returns all keys in the StoreNamespace
func (s *inMemoryStoreNamespace) GetKeys() []string {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	keys := []string{}
	for _, k := range s.originalCaseKeys {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of entries in the StoreNamespace
func (s *inMemoryStoreNamespace) Len() int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	return len(s.entries)
}

// GetAllValues from the StoreNamespace
func (s *inMemoryStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	copy := map[string]*base.OperatorIO{}
	counter := 0
	for lk, v := range s.entries {
		k := s.originalCaseKeys[lk]
		copy[k] = v.data
		counter++
		if limit != 0 && counter >= limit {
			return copy
		}
	}
	return copy
}

func matches(lk string, v StoreEntry, keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration, tnow time.Time) bool {
	if minAge != 0 && v.timestamp.Add(minAge).After(tnow) {
		return false
	}
	if maxAge != math.MaxInt64 && v.timestamp.Add(maxAge).Before(tnow) {
		return false
	}
	lkeyPattern := strings.ToLower(keyPattern)
	if keyPattern != "" && !strings.Contains(lk, lkeyPattern) {
		return false
	}
	if valuePattern != "" && !strings.Contains(v.data.GetString(), valuePattern) {
		return false
	}
	if modifiedByPattern != "" && !strings.Contains(v.modifiedBy.GetID(), modifiedByPattern) {
		return false
	}
	return true
}

// GetSearchResultWithMetadata searches through all keys, optionally finds substring in key, value and ID, and returns only records younger than maxAge
func (s *inMemoryStoreNamespace) GetSearchResultWithMetadata(keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration) map[string]StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	copy := map[string]StoreEntry{}
	for lk, v := range s.entries {
		if matches(lk, v, keyPattern, valuePattern, modifiedByPattern, minAge, maxAge, tnow) {
			k := s.originalCaseKeys[lk]
			copy[k] = v
		}
	}
	return copy
}

// DeleteOlder deletes records older than maxAge
func (s *inMemoryStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	deleteKeys := []string{}
	for k, md := range s.entries {
		ts := md.timestamp
		if ts.Add(maxAge).Before(tnow) {
			deleteKeys = append(deleteKeys, k)
		}
	}
	for _, k := range deleteKeys {
		s.deleteValueUnlocked(k)
	}
	return len(deleteKeys)
}

// Trim deletes all but the top k records sorted by timestamp
func (s *inMemoryStoreNamespace) Trim(k int) int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()

	if k >= len(s.entries) {
		return 0
	}

	// create an array of size k to store the top timestamps
	topK := utils.NewTopKList(k)
	deleteKeys := make([]string, 0, len(s.entries)-k)
	// iterate through the map and use insertion sort to find the top k timestamps
	for key, md := range s.entries {
		ts := md.timestamp
		cand := topK.Add(key, ts)
		if cand != nil {
			deleteKeys = append(deleteKeys, *cand)
		}
	}

	// delete all keys that are not in the top k
	for _, key := range deleteKeys {
		s.deleteValueUnlocked(key)
	}
	return len(deleteKeys)
}
