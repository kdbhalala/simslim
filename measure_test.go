package simslim

import "testing"

func TestMeasureTree(t *testing.T) {
	snap := psSnapshot{
		children: map[int][]int{
			10: {11, 12},
			11: {13},
			13: {10}, // A cycle must not double-count the root.
		},
		cpu: map[int]float64{10: 1.5, 11: 0.5, 12: 2, 13: 0},
	}
	footprint := map[int]int64{10: 100, 11: 20, 12: 30, 13: 4}

	got := measureTree(10, snap, footprint)
	if got.Processes != 4 {
		t.Errorf("Processes = %d, want 4", got.Processes)
	}
	if got.Bytes != 154 {
		t.Errorf("Bytes = %d, want 154", got.Bytes)
	}
	if got.CPU != 4 {
		t.Errorf("CPU = %v, want 4", got.CPU)
	}
}

func TestParsePS(t *testing.T) {
	out := "  PID  PPID %CPU COMM\n" +
		"   10     1  1.5 launchd_sim\n" +
		"   11    10 12.0 /usr/libexec/assistantd\n" +
		"   12    10  0.0 /System/Library/CoreServices/SpringBoard.app/SpringBoard\n" +
		"  bad line that should be skipped\n"

	snap := parsePS(out)
	if got := snap.children[10]; len(got) != 2 {
		t.Fatalf("children[10] = %v, want 2 entries", got)
	}
	if snap.comm[11] != "assistantd" { // basenamed from the full path
		t.Errorf("comm[11] = %q, want assistantd", snap.comm[11])
	}
	if snap.comm[12] != "SpringBoard" {
		t.Errorf("comm[12] = %q, want SpringBoard", snap.comm[12])
	}
	if snap.cpu[11] != 12 {
		t.Errorf("cpu[11] = %v, want 12", snap.cpu[11])
	}
	if _, ok := snap.comm[1]; ok {
		t.Errorf("header row was parsed as a process")
	}
}

func TestParseBytes(t *testing.T) {
	tests := map[string]int64{
		"512B":  512,
		"8K":    8 << 10,
		"823M":  823 << 20,
		"1.5G+": 3 << 29,
	}
	for input, want := range tests {
		if got := parseBytes(input); got != want {
			t.Errorf("parseBytes(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseFootprintDataConstDirty(t *testing.T) {
	affected := `======================================================================
sharingd [78446]: 64-bit    Footprint: 104 MB (16384 bytes per page)
======================================================================

  Dirty      Clean  Reclaimable    Regions    Category
    ---        ---          ---        ---    ---
  89 MB        0 B          0 B       1184    __DATA_CONST
2088 KB        0 B          0 B        913    __DATA_DIRTY
 103 MB    5472 KB          0 B       4742    TOTAL
`
	got, ok := parseFootprintDataConstDirty(affected)
	if !ok {
		t.Fatalf("affected sample: expected __DATA_CONST row to be found")
	}
	if got != 89<<20 {
		t.Errorf("affected sample: got %d, want %d", got, 89<<20)
	}
	if got <= sharedCacheUnsharedThreshold {
		t.Errorf("affected sample: %d should exceed threshold %d", got, sharedCacheUnsharedThreshold)
	}

	healthy := `  Dirty      Clean  Reclaimable    Regions    Category
    ---        ---          ---        ---    ---
 848 KB        0 B          0 B        120    __DATA_CONST
  12 MB        0 B          0 B        500    TOTAL
`
	got, ok = parseFootprintDataConstDirty(healthy)
	if !ok {
		t.Fatalf("healthy sample: expected __DATA_CONST row to be found")
	}
	if got != 848<<10 {
		t.Errorf("healthy sample: got %d, want %d", got, 848<<10)
	}
	if got > sharedCacheUnsharedThreshold {
		t.Errorf("healthy sample: %d should be below threshold %d", got, sharedCacheUnsharedThreshold)
	}

	missing := `  Dirty      Clean  Reclaimable    Regions    Category
    ---        ---          ---        ---    ---
2088 KB        0 B          0 B        913    __DATA_DIRTY
 103 MB    5472 KB          0 B       4742    TOTAL
`
	if got, ok := parseFootprintDataConstDirty(missing); ok || got != 0 {
		t.Errorf("missing sample: got (%d, %v), want (0, false)", got, ok)
	}
}
