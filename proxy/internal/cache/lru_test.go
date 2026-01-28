package cache

import (
	"testing"
)

func TestLRUIndex_AddAndExists(t *testing.T) {
	idx := newLRUIndex()

	// Initially empty
	if idx.exists("key1") {
		t.Error("key should not exist in empty index")
	}

	// Add item
	idx.add("key1", 100)

	if !idx.exists("key1") {
		t.Error("key should exist after adding")
	}

	if idx.size() != 1 {
		t.Errorf("expected size 1, got %d", idx.size())
	}
}

func TestLRUIndex_Access(t *testing.T) {
	idx := newLRUIndex()

	// Add multiple items
	idx.add("key1", 100)
	idx.add("key2", 200)
	idx.add("key3", 300)

	// Access key1 (making it most recent)
	idx.access("key1")

	// Evict should remove key2 (least recently used)
	key, size := idx.evict()
	if key != "key2" {
		t.Errorf("expected to evict key2, got %s", key)
	}
	if size != 200 {
		t.Errorf("expected size 200, got %d", size)
	}
}

func TestLRUIndex_Evict(t *testing.T) {
	idx := newLRUIndex()

	// Add items
	idx.add("key1", 100)
	idx.add("key2", 200)
	idx.add("key3", 300)

	// Evict (should remove key1, the oldest)
	key, size := idx.evict()
	if key != "key1" {
		t.Errorf("expected to evict key1, got %s", key)
	}
	if size != 100 {
		t.Errorf("expected size 100, got %d", size)
	}

	// Verify key1 no longer exists
	if idx.exists("key1") {
		t.Error("evicted key should not exist")
	}

	if idx.size() != 2 {
		t.Errorf("expected size 2 after eviction, got %d", idx.size())
	}
}

func TestLRUIndex_EvictEmpty(t *testing.T) {
	idx := newLRUIndex()

	// Evict from empty index
	key, size := idx.evict()
	if key != "" {
		t.Errorf("expected empty key, got %s", key)
	}
	if size != 0 {
		t.Errorf("expected size 0, got %d", size)
	}
}

func TestLRUIndex_Remove(t *testing.T) {
	idx := newLRUIndex()

	idx.add("key1", 100)
	idx.add("key2", 200)

	// Remove key1
	idx.remove("key1")

	if idx.exists("key1") {
		t.Error("removed key should not exist")
	}

	if idx.size() != 1 {
		t.Errorf("expected size 1 after removal, got %d", idx.size())
	}

	// Remove non-existent key (should not panic)
	idx.remove("nonexistent")
}

func TestLRUIndex_UpdateSize(t *testing.T) {
	idx := newLRUIndex()

	// Add item
	idx.add("key1", 100)

	// Update with new size
	idx.add("key1", 200)

	// Verify size updated
	if idx.size() != 1 {
		t.Errorf("expected size 1 after update, got %d", idx.size())
	}

	// Evict and check size
	key, size := idx.evict()
	if key != "key1" {
		t.Errorf("expected to evict key1, got %s", key)
	}
	if size != 200 {
		t.Errorf("expected updated size 200, got %d", size)
	}
}

func TestLRUIndex_LRUOrder(t *testing.T) {
	idx := newLRUIndex()

	// Add items in order
	idx.add("key1", 100)
	idx.add("key2", 200)
	idx.add("key3", 300)

	// Access key1 (making it most recent)
	idx.access("key1")

	// Evict should remove key2 (oldest unaccessed)
	key, _ := idx.evict()
	if key != "key2" {
		t.Errorf("expected to evict key2, got %s", key)
	}

	// Next evict should remove key3
	key, _ = idx.evict()
	if key != "key3" {
		t.Errorf("expected to evict key3, got %s", key)
	}

	// Last evict should remove key1
	key, _ = idx.evict()
	if key != "key1" {
		t.Errorf("expected to evict key1, got %s", key)
	}

	// Index should be empty
	if idx.size() != 0 {
		t.Errorf("expected empty index, got size %d", idx.size())
	}
}

func TestLRUIndex_MultipleAccess(t *testing.T) {
	idx := newLRUIndex()

	// Add items
	idx.add("key1", 100)
	idx.add("key2", 200)
	idx.add("key3", 300)
	idx.add("key4", 400)

	// Access pattern: key2, key1, key3 (key4 is least recent)
	idx.access("key2")
	idx.access("key1")
	idx.access("key3")

	// Evict should remove key4
	key, _ := idx.evict()
	if key != "key4" {
		t.Errorf("expected to evict key4, got %s", key)
	}
}

func TestLRUIndex_AccessNonExistent(t *testing.T) {
	idx := newLRUIndex()

	// Access non-existent key (should not panic)
	idx.access("nonexistent")

	if idx.size() != 0 {
		t.Errorf("expected size 0, got %d", idx.size())
	}
}
