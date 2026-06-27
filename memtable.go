package cascade

import "github.com/egregors/sortedmap"

const MemtableMaxBytes = 1024 // 1 KB

type Memtable struct {
	data sortedmap.SortedMap[map[string]KVEntry, string, KVEntry]
	size int
	/*
		What data do we need to solve the following problems?
		1. How do we keep track of how full the memtable is?
	*/
}

func Less(i, j sortedmap.KV[string, KVEntry]) bool {
	return i.Key < j.Key
}

func NewMemtable() *Memtable {
	return &Memtable{
		data: *sortedmap.New[map[string]KVEntry, string, KVEntry](Less),
	}
}

func (m *Memtable) Get(key string) (KVEntry, bool) {
	return m.data.Get(key)
}

func (m *Memtable) Set(key string, entry KVEntry) {
	// to reduce prev size and increase new size
	prevEntry, _ := m.data.Get(key)

	prevSizeWithoutTombstone := len(prevEntry.Key) + len(prevEntry.Value)

	m.size += (len(entry.Key) + len(entry.Value) - prevSizeWithoutTombstone)

	m.data.Insert(key, entry)
}

func (m *Memtable) IsFull() bool { return m.size == MemtableMaxBytes }

func (m *Memtable) Size() int { return m.size }

func (m *Memtable) SortedEntries() []KVEntry { return m.data.CollectValues() }
