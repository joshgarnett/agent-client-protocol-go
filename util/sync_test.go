package util

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncSlice(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		ss := NewSyncSlice[int]()

		// Test initial state
		assert.Equal(t, 0, ss.Len())
		assert.Nil(t, ss.GetAll())

		// Test append
		ss.Append(1)
		ss.Append(2)
		ss.Append(3)

		assert.Equal(t, 3, ss.Len())
		assert.Equal(t, []int{1, 2, 3}, ss.GetAll())

		// Test get
		val, ok := ss.Get(1)
		assert.True(t, ok)
		assert.Equal(t, 2, val)

		// Test get invalid index
		val, ok = ss.Get(5)
		assert.False(t, ok)
		assert.Equal(t, 0, val) // zero value

		// Test remove
		assert.True(t, ss.Remove(1))
		assert.Equal(t, []int{1, 3}, ss.GetAll())

		// Test remove invalid index
		assert.False(t, ss.Remove(5))

		// Test clear
		ss.Clear()
		assert.Equal(t, 0, ss.Len())
		assert.Nil(t, ss.GetAll())
	})

	t.Run("concurrent access", func(t *testing.T) {
		ss := NewSyncSlice[int]()

		var wg sync.WaitGroup

		// Multiple goroutines appending
		for i := range 10 {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				ss.Append(val)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 10, ss.Len())

		// Multiple goroutines reading
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ss.GetAll()
				ss.Len()
			}()
		}

		wg.Wait()
	})
}

func TestCallbackRegistry(t *testing.T) {
	type testCallback func(int) error

	t.Run("basic operations", func(t *testing.T) {
		cr := NewCallbackRegistry[testCallback]()

		// Test initial state
		assert.Equal(t, 0, cr.Len())
		assert.Nil(t, cr.GetAll())

		callback1 := func(int) error { return nil }
		callback2 := func(int) error { return nil }
		callback3 := func(int) error { return nil }

		// Test register
		cr.Register(callback1)
		cr.Register(callback2)
		cr.Register(callback3)

		assert.Equal(t, 3, cr.Len())
		callbacks := cr.GetAll()
		assert.Len(t, callbacks, 3)

		// Test unregister
		assert.True(t, cr.Unregister(1))
		assert.Equal(t, 2, cr.Len())

		// Test unregister invalid index
		assert.False(t, cr.Unregister(5))

		// Test clear
		cr.Clear()
		assert.Equal(t, 0, cr.Len())
		assert.Nil(t, cr.GetAll())
	})

	t.Run("concurrent access", func(t *testing.T) {
		cr := NewCallbackRegistry[testCallback]()

		var wg sync.WaitGroup

		// Multiple goroutines registering callbacks
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				callback := func(int) error { return nil }
				cr.Register(callback)
			}()
		}

		wg.Wait()
		assert.Equal(t, 10, cr.Len())

		// Multiple goroutines reading
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cr.GetAll()
				cr.Len()
			}()
		}

		wg.Wait()
	})
}

func TestSyncSlice_StringType(t *testing.T) {
	ss := NewSyncSlice[string]()

	ss.Append("hello")
	ss.Append("world")

	val, ok := ss.Get(0)
	assert.True(t, ok)
	assert.Equal(t, "hello", val)

	all := ss.GetAll()
	assert.Equal(t, []string{"hello", "world"}, all)
}

func TestCallbackRegistry_StringType(t *testing.T) {
	cr := NewCallbackRegistry[string]()

	cr.Register("callback1")
	cr.Register("callback2")

	assert.Equal(t, 2, cr.Len())

	all := cr.GetAll()
	assert.Equal(t, []string{"callback1", "callback2"}, all)
}

func TestAtomicValue(t *testing.T) {
	t.Run("basic operations", func(t *testing.T) {
		av := NewAtomicValue("initial")

		// Test Load
		assert.Equal(t, "initial", av.Load())

		// Test Store
		av.Store("updated")
		assert.Equal(t, "updated", av.Load())

		// Test Swap
		old := av.Swap("swapped")
		assert.Equal(t, "updated", old)
		assert.Equal(t, "swapped", av.Load())

		// Test CompareAndSwap success
		assert.True(t, av.CompareAndSwap("swapped", "cas_success"))
		assert.Equal(t, "cas_success", av.Load())

		// Test CompareAndSwap failure
		assert.False(t, av.CompareAndSwap("wrong", "cas_fail"))
		assert.Equal(t, "cas_success", av.Load())
	})

	t.Run("LoadOrStore", func(t *testing.T) {
		av := NewAtomicValue("initial")

		// Load existing value
		result, loaded := av.LoadOrStore("new_value")
		assert.True(t, loaded)
		assert.Equal(t, "initial", result)

		// Create new AtomicValue for store test
		av2 := &AtomicValue[string]{}
		result, loaded = av2.LoadOrStore("stored")
		assert.False(t, loaded)
		assert.Equal(t, "stored", result)
		assert.Equal(t, "stored", av2.Load())
	})

	t.Run("Update", func(t *testing.T) {
		av := NewAtomicValue(10)

		// Test Update function
		result := av.Update(func(old int) int {
			return old * 2
		})
		assert.Equal(t, 20, result)
		assert.Equal(t, 20, av.Load())

		// Test concurrent updates
		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				av.Update(func(old int) int {
					return old + 1
				})
			}()
		}
		wg.Wait()
		assert.Equal(t, 30, av.Load()) // 20 + 10 increments
	})

	t.Run("complex types", func(t *testing.T) {
		type testStruct struct {
			Name  string
			Value int
		}

		initial := testStruct{Name: "initial", Value: 42}
		av := NewAtomicValue(initial)

		assert.Equal(t, initial, av.Load())

		updated := testStruct{Name: "updated", Value: 100}
		av.Store(updated)
		assert.Equal(t, updated, av.Load())

		// Test coordinated update
		result := av.Update(func(old testStruct) testStruct {
			return testStruct{
				Name:  old.Name + "_modified",
				Value: old.Value * 2,
			}
		})
		expected := testStruct{Name: "updated_modified", Value: 200}
		assert.Equal(t, expected, result)
		assert.Equal(t, expected, av.Load())
	})

	t.Run("zero value", func(t *testing.T) {
		// Test uninitialized AtomicValue returns zero value
		av := &AtomicValue[string]{}
		assert.Empty(t, av.Load())

		av2 := &AtomicValue[int]{}
		assert.Equal(t, 0, av2.Load())

		// Test Swap with zero value
		old := av.Swap("first")
		assert.Empty(t, old)
		assert.Equal(t, "first", av.Load())
	})

	t.Run("concurrent access", func(t *testing.T) {
		av := NewAtomicValue(0)
		var wg sync.WaitGroup

		// Multiple goroutines storing values
		for i := range 100 {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				av.Store(val)
			}(i)
		}

		// Multiple goroutines reading values
		for range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				av.Load()
			}()
		}

		wg.Wait()
		// Just ensure no races, value could be anything
		result := av.Load()
		assert.True(t, result >= 0 && result < 100)
	})
}
