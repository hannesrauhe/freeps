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
	entries map[string]StoreEntry
	nsLock  sync.Mutex
}

var _ StoreNamespace = &inMemoryStoreNamespace{}

// GetValue from the StoreNamespace
func (s *inMemoryStoreNamespace) GetValue(key string) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	io, ok := s.entries[key]
	if !ok {
		return base.MakeOutputError(http.StatusNotFound, "Key not found")
	}
	return io.data
}

// GetValueBeforeExpiration gets the value from the StoreNamespace, but returns error if older than maxAge
func (s *inMemoryStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	ent, ok := s.entries[key]
	if !ok {
		return base.MakeOutputError(http.StatusNotFound, "Key not found")
	}
	ts := ent.timestamp
	if ts.Add(maxAge).Before(time.Now()) {
		return base.MakeOutputError(http.StatusGone, "key is older than %v", maxAge)
	}
	return ent.data
}

func (s *inMemoryStoreNamespace) setValueUnlocked(key string, newValue *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	s.entries[key] = StoreEntry{newValue, time.Now(), modifiedBy}
	return base.MakeEmptyOutput()
}

func (s *inMemoryStoreNamespace) deleteValueUnlocked(key string) {
	delete(s.entries, key)
}

// SetValue in the StoreNamespace
func (s *inMemoryStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy string) error {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	s.setValueUnlocked(key, io, modifiedBy)
	return nil
}

// CompareAndSwap sets the value if the string representation of the already stored value is as expected
func (s *inMemoryStoreNamespace) CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	oldV, exists := s.entries[key]
	if !exists {
		return base.MakeOutputError(http.StatusNotFound, "key does not exist yet")
	}
	if oldV.data == nil || oldV.data.GetString() != expected {
		return base.MakeOutputError(http.StatusConflict, "old value is different from expectation")
	}
	return s.setValueUnlocked(key, newValue, modifiedBy)
}

// OverwriteValueIfOlder sets the value only if the key does not exist or has been written before maxAge
func (s *inMemoryStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy string) *base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	n := time.Now()
	md, keyExists := s.entries[key]
	if keyExists && md.timestamp.Add(maxAge).After(n) {
		return base.MakeOutputError(http.StatusConflict, "%v already exists and is only %v old", key, n.Sub(md.timestamp))
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
	for k := range s.entries {
		keys = append(keys, k)
	}
	return keys
}

// GetAllValues from the StoreNamespace
func (s *inMemoryStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	copy := map[string]*base.OperatorIO{}
	counter := 0
	for k, v := range s.entries {
		copy[k] = v.data
		counter++
		if limit != 0 && counter >= limit {
			return copy
		}
	}
	return copy
}

func matches(k string, v StoreEntry, keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration, tnow time.Time) bool {
	if minAge != 0 && v.timestamp.Add(minAge).After(tnow) {
		return false
	}
	if maxAge != math.MaxInt64 && v.timestamp.Add(maxAge).Before(tnow) {
		return false
	}
	if keyPattern != "" && !strings.Contains(k, keyPattern) {
		return false
	}
	if valuePattern != "" && !strings.Contains(v.data.GetString(), valuePattern) {
		return false
	}
	if modifiedByPattern != "" && !strings.Contains(v.modifiedBy, modifiedByPattern) {
		return false
	}
	return true
}

// GetAllFiltered searches through all keys, optionally finds substring in key, value and ID, and returns only records younger than maxAge
func (s *inMemoryStoreNamespace) GetAllFiltered(keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration) map[string]*base.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	copy := map[string]*base.OperatorIO{}
	for k, v := range s.entries {
		if matches(k, v, keyPattern, valuePattern, modifiedByPattern, minAge, maxAge, tnow) {
			copy[k] = v.data
		}
	}
	return copy
}

// GetSearchResultWithMetadata searches through all keys, optionally finds substring in key, value and ID, and returns only records younger than maxAge
func (s *inMemoryStoreNamespace) GetSearchResultWithMetadata(keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration) map[string]StoreEntry {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	copy := map[string]StoreEntry{}
	for k, v := range s.entries {
		if matches(k, v, keyPattern, valuePattern, modifiedByPattern, minAge, maxAge, tnow) {
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

// DeleteOlderThanMaxSize deletes all but the top k records sorted by timestamp
func (s *inMemoryStoreNamespace) DeleteOlderThanMaxSize(k int) int {
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
