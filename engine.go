package cascade

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
)

var ErrNotFound = errors.New("not found")

// Level size thresholds that trigger compaction (bytes).
const (
	L0MaxBytes = 10 * 1024   // 10 KB
	L1MaxBytes = 100 * 1024  // 100 KB
	L2MaxBytes = 1024 * 1024 // 1 MB
)

// Snapshot struct that captures in-memory, point in time disk state
type Snapshot struct {
	snapshotID uint64
	l0         []*SSTable // unsorted; newest-first on reads
	l1         []*SSTable // sorted by key range
	l2         []*SSTable // sorted by key range
}

type Engine struct {
	// Core data path
	memtable     *Memtable
	currSnapshot Snapshot
	highSeqNo    atomic.Uint64

	dataDir string

	ioCounter *IOCounter
}

func NewEngine(dataDir string) (*Engine, error) {
	err := os.MkdirAll(dataDir, os.ModeAppend)
	if err != nil {
		return nil, err
	}

	engine := &Engine{
		memtable:     NewMemtable(),
		currSnapshot: Snapshot{},
		dataDir:      dataDir,
		ioCounter:    NewIOCounter(),
	}

	return engine, nil
}

func (e *Engine) Get(key string) ([]byte, error) {
	// read memtable
	entry, ok := e.memtable.Get(key)
	if ok {
		if entry.IsTombstone {
			return nil, ErrNotFound
		}

		return []byte(entry.Value), nil
	}

	// TODO: read immutable memtable

	// read sstable in order

	// read l0
	for ssIdx := range e.currSnapshot.l0 {
		entry, found, err := e.currSnapshot.l0[len(e.currSnapshot.l0)-ssIdx-1].Get(key, e.ioCounter)
		if err != nil {
			return nil, err
		}

		if found {
			if entry.IsTombstone {
				return nil, ErrNotFound
			}

			return []byte(entry.Value), nil
		}
	}

	//TODO:read l1

	//TODO:read l2

	return nil, ErrNotFound
}

func (e *Engine) Upsert(key string, value []byte) error {
	entry := GenerateUpsert(key, string(value))

	if e.memtable.IsFull() {
		e.Flush()
	}

	e.memtable.Set(key, entry)

	return nil
}

func (e *Engine) Delete(key string) error {
	entry := GenerateDelete(key)

	e.memtable.Set(key, entry)

	return nil
}

// Flush writes the immutable memtable to a new L0 SSTable.
func (e *Engine) Flush() error {
	// encode the current table into sstable
	oldMemTableRef := e.memtable

	// create new memtable
	e.memtable = NewMemtable()

	e.highSeqNo.Add(1)

	filepath := path.Join(e.dataDir, fmt.Sprintf("sstable.%d.data", e.highSeqNo.Load()))

	sstable, err := WriteSSTable(e.highSeqNo.Load(), filepath, oldMemTableRef.SortedEntries())
	if err != nil {
		return err
	}

	// push to disk as sstable to l0
	e.currSnapshot.l0 = append(e.currSnapshot.l0, sstable)

	return nil
}

// Sync serializes a Checkpoint to disk so the engine can recover after a restart.
func (e *Engine) Sync() error { return nil }

// Recover reads the latest Checkpoint from disk and restores engine state.
func (e *Engine) Recover() error { return nil }

// Restart resets all in-memory state and replays the last Checkpoint from disk,
// simulating a clean process restart without constructing a new Engine.
func (e *Engine) Restart() error { return nil }

// Compact runs a full compaction pass across all levels.
func (e *Engine) Compact() error { return nil }

func (e *Engine) IOCount() int64        { return 0 }
func (e *Engine) ResetIOCount()         {}
func (e *Engine) GetCurrentDir() string { return e.dataDir }

func (e *Engine) l0Dir() string          { return filepath.Join(e.dataDir, "l0") }
func (e *Engine) l1Dir() string          { return filepath.Join(e.dataDir, "l1") }
func (e *Engine) l2Dir() string          { return filepath.Join(e.dataDir, "l2") }
func (e *Engine) checkpointPath() string { return filepath.Join(e.dataDir, "checkpoint.json") }
