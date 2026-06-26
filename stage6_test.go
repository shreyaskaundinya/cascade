package cascade_test

// Stage 6: Levelled Compaction End-to-End
// Goal: wire compaction into the engine, run a full pass across L0 → L1 → L2,
// and observe the reduction in read IOs as scattered SSTables are merged.
// Run with: go test -run TestStage6 ./...

import (
	"fmt"
	"testing"

	cascade "github.com/anirudhRowjee/cascade"
)

// After compaction all keys written before it must still be readable.
func TestStage6_CompactionPreservesAllKeys(t *testing.T) {
	e := newTestEngine(t)

	const n = 20
	value := make([]byte, 64)
	for i := range n {
		_ = e.Upsert(fmt.Sprintf("key-%03d", i), value)
		_ = e.Flush()
	}

	if err := e.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	for i := range n {
		key := fmt.Sprintf("key-%03d", i)
		if _, err := e.Get(key); err != nil {
			t.Errorf("Get %q after compaction: %v", key, err)
		}
	}
}

// A key deleted before compaction must remain absent after compaction.
func TestStage6_CompactionRespectsTombstones(t *testing.T) {
	e := newTestEngine(t)

	_ = e.Upsert("gone", []byte("here"))
	_ = e.Flush()
	_ = e.Delete("gone")
	_ = e.Flush()

	if err := e.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	_, err := e.Get("gone")
	if err != cascade.ErrNotFound {
		t.Fatalf("want ErrNotFound after compaction, got %v", err)
	}
}
