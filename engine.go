package cascade

import (
	"errors"
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

// Checkpoint captures all state needed to fully recover the engine after a restart.
type Checkpoint struct {
	L0Paths  []string  // SSTable paths in L0, newest first
	L1Paths  []string  // SSTable paths in L1
	L2Paths  []string  // SSTable paths in L2
	Memtable []KVEntry // snapshot of the active memtable at checkpoint time
}

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
	return nil, errors.New("This shouldn't fail! Fix me!!!!!")
}

func (e *Engine) Get(key string) ([]byte, error)        { return nil, ErrNotFound }
func (e *Engine) Upsert(key string, value []byte) error { return nil }
func (e *Engine) Delete(key string) error               { return nil }

// Flush writes the immutable memtable to a new L0 SSTable.
func (e *Engine) Flush() error { return nil }

// Sync serializes a Checkpoint to disk so the engine can recover after a restart.
func (e *Engine) Sync() error { return nil }

// Recover reads the latest Checkpoint from disk and restores engine state.
func (e *Engine) Recover() error { return nil }

// Restart resets all in-memory state and replays the last Checkpoint from disk,
// simulating a clean process restart without constructing a new Engine.
func (e *Engine) Restart() error { return nil }

// Compact runs a full compaction pass across all levels.
func (e *Engine) Compact() error { return nil }

func (e *Engine) IOCount() int64    { return 0 }
func (e *Engine) ResetIOCount()     {}
func (e *Engine) GetCurrentDir() string { return e.dataDir }

func (e *Engine) l0Dir() string          { return filepath.Join(e.dataDir, "l0") }
func (e *Engine) l1Dir() string          { return filepath.Join(e.dataDir, "l1") }
func (e *Engine) l2Dir() string          { return filepath.Join(e.dataDir, "l2") }
func (e *Engine) checkpointPath() string { return filepath.Join(e.dataDir, "checkpoint.json") }
