package freepsstore

import (
	"time"

	"github.com/hannesrauhe/freeps/base"
)

// NullStoreNamespace behaves like the InMemoryNamespace but doesn't actually store anything (/dev/null)
type NullStoreNamespace struct {
}

var _ StoreNamespace = &NullStoreNamespace{}

// GetValue from the StoreNamespace
func (s *NullStoreNamespace) GetValue(key string) StoreEntry {
	return NotFoundEntry
}

// GetValueBeforeExpiration gets the value from the StoreNamespace, but returns error if older than maxAge
func (s *NullStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) StoreEntry {
	return NotFoundEntry
}

// SetValue in the StoreNamespace
func (s *NullStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	return StoreEntry{io, time.Now(), modifiedBy}
}

// SetAll sets all values in the StoreNamespace
func (s *NullStoreNamespace) SetAll(valueMap map[string]interface{}, modifiedBy *base.Context) *base.OperatorIO {
	return base.MakeEmptyOutput()
}

// CompareAndSwap sets the value if the string representation of the already stored value is as expected
func (s *NullStoreNamespace) CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	return NotFoundEntry
}

// UpdateTransaction updates the value in the StoreNamespace by calling the function fn with the current value
func (s *NullStoreNamespace) UpdateTransaction(key string, fn func(StoreEntry) *base.OperatorIO, modifiedBy *base.Context) StoreEntry {
	return s.SetValue(key, fn(s.GetValue(key)), modifiedBy)
}

// OverwriteValueIfOlder sets the value only if the key does not exist or has been written before maxAge
func (s *NullStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy *base.Context) StoreEntry {
	return StoreEntry{io, time.Now(), modifiedBy}
}

// DeleteValue from the StoreNamespace
func (s *NullStoreNamespace) DeleteValue(key string) {

}

// GetKeys returns all keys in the StoreNamespace
func (s *NullStoreNamespace) GetKeys() []string {
	keys := []string{}
	return keys
}

// Len returns the number of entries in the StoreNamespace
func (s *NullStoreNamespace) Len() int {
	return 0
}

// GetAllValues from the StoreNamespace
func (s *NullStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	copy := map[string]*base.OperatorIO{}
	return copy
}

// GetSearchResultWithMetadata searches through all keys, optionally finds substring in key, value and ID, and returns only records younger than maxAge
func (s *NullStoreNamespace) GetSearchResultWithMetadata(keyPattern, valuePattern, modifiedByPattern string, minAge, maxAge time.Duration) map[string]StoreEntry {
	copy := map[string]StoreEntry{}
	return copy
}

// DeleteOlder deletes records older than maxAge
func (s *NullStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	return 0
}

// DeleteOlderThanMaxSize deletes all but the top k records sorted by timestamp
func (s *NullStoreNamespace) DeleteOlderThanMaxSize(k int) int {
	return 0
}

func (s *NullStoreNamespace) Trim(numEntries int) int {
	return 0
}
