package util

import "sync"

// SyncMap is a thread-safe generic map implementation.
type SyncMap[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// NewSyncMap creates a new SyncMap instance.
func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{
		data: make(map[K]V),
	}
}

// Store adds the element to the map even if it does exist.
func (sm *SyncMap[K, V]) Store(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.data[key] = value
}

// Load retrieves the element in the map, returning the element and bool.
// The bool is false if the element doesn't exist.
func (sm *SyncMap[K, V]) Load(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	value, exists := sm.data[key]
	return value, exists
}

// Delete deletes the element in the map.
func (sm *SyncMap[K, V]) Delete(key K) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.data, key)
}

// Clear resets the underlying map.
func (sm *SyncMap[K, V]) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.data = make(map[K]V)
}

// Replace replaces the underlying map with a map passed in.
func (sm *SyncMap[K, V]) Replace(newMap map[K]V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.data = make(map[K]V, len(newMap))
	for k, v := range newMap {
		sm.data[k] = v
	}
}

// GetAll returns a new copy of the underlying map.
func (sm *SyncMap[K, V]) GetAll() map[K]V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[K]V, len(sm.data))
	for k, v := range sm.data {
		result[k] = v
	}
	return result
}

// Count returns the number of key-value pairs in the map.
func (sm *SyncMap[K, V]) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.data)
}

// CompareAndDelete deletes the entry for a key if its value matches the old value.
// Returns true if deletion occurred.
func (sm *SyncMap[K, V]) CompareAndDelete(key K, old V) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	current, exists := sm.data[key]
	if !exists {
		return false
	}

	// Use comparable interface for equality check
	if any(current) == any(old) {
		delete(sm.data, key)
		return true
	}
	return false
}

// CompareAndSwap swaps the old and new values for a key if the current value matches old.
// Returns true if swap was successful.
func (sm *SyncMap[K, V]) CompareAndSwap(key K, old, newValue V) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	current, exists := sm.data[key]
	if !exists {
		return false
	}

	// Use comparable interface for equality check
	if any(current) == any(old) {
		sm.data[key] = newValue
		return true
	}
	return false
}

// LoadAndDelete removes the value for a key and returns the previous value.
// Returns the value and a boolean indicating if a value was loaded.
func (sm *SyncMap[K, V]) LoadAndDelete(key K) (V, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	value, exists := sm.data[key]
	if exists {
		delete(sm.data, key)
	}
	return value, exists
}

// LoadOrStore returns existing value if present, otherwise stores and returns the new value.
// Returns the actual value and a boolean indicating if it was loaded (true) or stored (false).
func (sm *SyncMap[K, V]) LoadOrStore(key K, value V) (V, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if existing, exists := sm.data[key]; exists {
		return existing, true
	}
	sm.data[key] = value
	return value, false
}

// Swap swaps the value for a key and returns the previous value.
// Returns the previous value and a boolean indicating if a value was loaded.
func (sm *SyncMap[K, V]) Swap(key K, value V) (V, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	previous, exists := sm.data[key]
	sm.data[key] = value
	return previous, exists
}

// SyncSlice is a thread-safe generic slice implementation.
type SyncSlice[T any] struct {
	mu   sync.RWMutex
	data []T
}

// NewSyncSlice creates a new SyncSlice instance.
func NewSyncSlice[T any]() *SyncSlice[T] {
	return &SyncSlice[T]{
		data: make([]T, 0),
	}
}

// Append adds an element to the end of the slice.
func (ss *SyncSlice[T]) Append(value T) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.data = append(ss.data, value)
}

// Get retrieves the element at the specified index.
// Returns the element and a boolean indicating if the index is valid.
func (ss *SyncSlice[T]) Get(index int) (T, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var zero T
	if index < 0 || index >= len(ss.data) {
		return zero, false
	}
	return ss.data[index], true
}

// GetAll returns a copy of all elements in the slice.
func (ss *SyncSlice[T]) GetAll() []T {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if len(ss.data) == 0 {
		return nil
	}

	result := make([]T, len(ss.data))
	copy(result, ss.data)
	return result
}

// Clear removes all elements from the slice.
func (ss *SyncSlice[T]) Clear() {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	ss.data = ss.data[:0]
}

// Len returns the length of the slice.
func (ss *SyncSlice[T]) Len() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	return len(ss.data)
}

// Remove removes the element at the specified index.
// Returns true if the element was removed, false if the index is invalid.
func (ss *SyncSlice[T]) Remove(index int) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if index < 0 || index >= len(ss.data) {
		return false
	}

	ss.data = append(ss.data[:index], ss.data[index+1:]...)
	return true
}

// CallbackRegistry is a thread-safe registry for callback functions.
type CallbackRegistry[T any] struct {
	mu        sync.RWMutex
	callbacks []T
}

// NewCallbackRegistry creates a new CallbackRegistry instance.
func NewCallbackRegistry[T any]() *CallbackRegistry[T] {
	return &CallbackRegistry[T]{
		callbacks: make([]T, 0),
	}
}

// Register adds a callback to the registry.
func (cr *CallbackRegistry[T]) Register(callback T) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.callbacks = append(cr.callbacks, callback)
}

// Unregister removes a callback from the registry by index.
// Returns true if the callback was removed, false if the index is invalid.
func (cr *CallbackRegistry[T]) Unregister(index int) bool {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if index < 0 || index >= len(cr.callbacks) {
		return false
	}

	cr.callbacks = append(cr.callbacks[:index], cr.callbacks[index+1:]...)
	return true
}

// GetAll returns a copy of all registered callbacks.
func (cr *CallbackRegistry[T]) GetAll() []T {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	if len(cr.callbacks) == 0 {
		return nil
	}

	result := make([]T, len(cr.callbacks))
	copy(result, cr.callbacks)
	return result
}

// Clear removes all callbacks from the registry.
func (cr *CallbackRegistry[T]) Clear() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.callbacks = cr.callbacks[:0]
}

// Len returns the number of registered callbacks.
func (cr *CallbackRegistry[T]) Len() int {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return len(cr.callbacks)
}

// AtomicValue provides type-safe concurrent operations for types not handled by sync/atomic.
// For primitive types (bool, int32, int64, uint32, uint64, uintptr, unsafe.Pointer),
// use the corresponding atomic types from sync/atomic directly.
type AtomicValue[T any] struct {
	mu    sync.RWMutex
	value T
	set   bool
}

// NewAtomicValue creates a new AtomicValue with the given initial value.
func NewAtomicValue[T any](initial T) *AtomicValue[T] {
	return &AtomicValue[T]{
		value: initial,
		set:   true,
	}
}

// Store stores the given value.
func (av *AtomicValue[T]) Store(v T) {
	av.mu.Lock()
	defer av.mu.Unlock()
	av.value = v
	av.set = true
}

// Load loads and returns the stored value.
// Returns the zero value of T if no value has been stored.
func (av *AtomicValue[T]) Load() T {
	av.mu.RLock()
	defer av.mu.RUnlock()
	if av.set {
		return av.value
	}
	var zero T
	return zero
}

// Swap stores the new value and returns the previous value.
// Returns the zero value of T if no value was previously stored.
func (av *AtomicValue[T]) Swap(newValue T) T {
	av.mu.Lock()
	defer av.mu.Unlock()

	var old T
	if av.set {
		old = av.value
	}
	av.value = newValue
	av.set = true
	return old
}

// CompareAndSwap compares the stored value with old and,
// if they are equal, stores newValue. Returns true if the swap was successful.
func (av *AtomicValue[T]) CompareAndSwap(old, newValue T) bool {
	av.mu.Lock()
	defer av.mu.Unlock()

	// If not set, only succeed if old is zero value
	if !av.set {
		var zero T
		if any(old) != any(zero) {
			return false
		}
		av.value = newValue
		av.set = true
		return true
	}

	// Compare current value with old
	if any(av.value) == any(old) {
		av.value = newValue
		return true
	}
	return false
}

// LoadOrStore loads the stored value if it exists, or stores and returns
// the given value if no value was stored. The loaded result is true if the value
// was loaded, false if stored.
func (av *AtomicValue[T]) LoadOrStore(newValue T) (T, bool) {
	av.mu.Lock()
	defer av.mu.Unlock()

	if av.set {
		return av.value, true
	}

	av.value = newValue
	av.set = true
	return newValue, false
}

// Update updates the stored value using the provided function.
// The function is called with the current value and should return the new value.
// Returns the new value after the update.
func (av *AtomicValue[T]) Update(updateFunc func(T) T) T {
	av.mu.Lock()
	defer av.mu.Unlock()

	var current T
	if av.set {
		current = av.value
	}

	av.value = updateFunc(current)
	av.set = true
	return av.value
}
