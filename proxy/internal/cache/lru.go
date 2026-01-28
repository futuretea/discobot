package cache

import (
	"container/list"
	"time"
)

// lruIndex tracks cached items for LRU eviction.
type lruIndex struct {
	items map[string]*lruItem // key -> item
	list  *list.List          // LRU list (least recent at front)
}

// lruItem represents an entry in the LRU index.
type lruItem struct {
	key      string
	size     int64
	lastUsed time.Time
	element  *list.Element
}

// newLRUIndex creates a new LRU index.
func newLRUIndex() *lruIndex {
	return &lruIndex{
		items: make(map[string]*lruItem),
		list:  list.New(),
	}
}

// add adds or updates an item in the index.
func (idx *lruIndex) add(key string, size int64) {
	now := time.Now()

	if item, exists := idx.items[key]; exists {
		// Update existing item
		item.size = size
		item.lastUsed = now
		idx.list.MoveToBack(item.element)
	} else {
		// Add new item
		item := &lruItem{
			key:      key,
			size:     size,
			lastUsed: now,
		}
		item.element = idx.list.PushBack(item)
		idx.items[key] = item
	}
}

// access marks an item as recently used.
func (idx *lruIndex) access(key string) {
	if item, exists := idx.items[key]; exists {
		item.lastUsed = time.Now()
		idx.list.MoveToBack(item.element)
	}
}

// exists checks if a key exists in the index.
func (idx *lruIndex) exists(key string) bool {
	_, exists := idx.items[key]
	return exists
}

// remove removes an item from the index.
func (idx *lruIndex) remove(key string) {
	if item, exists := idx.items[key]; exists {
		idx.list.Remove(item.element)
		delete(idx.items, key)
	}
}

// evict removes and returns the least recently used item.
// Returns empty string and 0 if no items exist.
func (idx *lruIndex) evict() (key string, size int64) {
	element := idx.list.Front()
	if element == nil {
		return "", 0
	}

	item := element.Value.(*lruItem)
	idx.list.Remove(element)
	delete(idx.items, item.key)

	return item.key, item.size
}

// size returns the number of items in the index.
func (idx *lruIndex) size() int {
	return len(idx.items)
}
