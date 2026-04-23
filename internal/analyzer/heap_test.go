package analyzer

import (
	"testing"

	"github.com/google/pprof/profile"
)

func makeHeapProfile(samples []struct {
	fn    string
	file  string
	alloc int64
}) *profile.Profile {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_objects", Unit: "count"},
			{Type: "alloc_space", Unit: "bytes"},
		},
	}
	for _, s := range samples {
		fn := &profile.Function{ID: uint64(len(p.Function)) + 1, Name: s.fn, Filename: s.file}
		p.Function = append(p.Function, fn)
		loc := &profile.Location{
			ID:   uint64(len(p.Location)) + 1,
			Line: []profile.Line{{Function: fn}},
		}
		p.Location = append(p.Location, loc)
		p.Sample = append(p.Sample, &profile.Sample{
			Value:    []int64{1, s.alloc},
			Location: []*profile.Location{loc},
		})
	}
	return p
}

func TestAnalyzeHeap_TopN(t *testing.T) {
	p := makeHeapProfile([]struct {
		fn    string
		file  string
		alloc int64
	}{
		{"pkg.AllocBig", "pkg/big.go", 10 * 1024 * 1024},
		{"pkg.AllocSmall", "pkg/small.go", 512 * 1024},
		{"pkg.NewFoo", "pkg/foo.go", 2 * 1024 * 1024},
	})

	rep, err := AnalyzeHeap(p, 2)
	if err != nil {
		t.Fatalf("AnalyzeHeap: %v", err)
	}
	if len(rep.TopAllocators) != 2 {
		t.Errorf("expected 2 top allocators, got %d", len(rep.TopAllocators))
	}
	if rep.TopAllocators[0].Function != "pkg.AllocBig" {
		t.Errorf("expected pkg.AllocBig at top, got %s", rep.TopAllocators[0].Function)
	}
}

func TestAnalyzeHeap_Percentages(t *testing.T) {
	p := makeHeapProfile([]struct {
		fn    string
		file  string
		alloc int64
	}{
		{"pkg.Heavy", "pkg/heavy.go", 800},
		{"pkg.Light", "pkg/light.go", 200},
	})

	rep, err := AnalyzeHeap(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeHeap: %v", err)
	}
	if rep.TotalBytes != 1000 {
		t.Errorf("expected TotalBytes=1000, got %d", rep.TotalBytes)
	}
	for _, e := range rep.TopAllocators {
		if e.Function == "pkg.Heavy" && (e.FlatPct < 79 || e.FlatPct > 81) {
			t.Errorf("expected ~80%% for pkg.Heavy, got %.1f%%", e.FlatPct)
		}
	}
}

func TestAnalyzeHeap_Recommendations(t *testing.T) {
	p := makeHeapProfile([]struct {
		fn    string
		file  string
		alloc int64
	}{
		{"encoding/json.Marshal", "json/encode.go", 1_800_000_000},
		{"pkg.Other", "pkg/other.go", 1_000_000},
	})

	rep, err := AnalyzeHeap(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeHeap: %v", err)
	}
	for _, e := range rep.TopAllocators {
		if e.Function == "encoding/json.Marshal" && e.Recommend == "" {
			t.Error("expected recommendation for json.Marshal, got empty")
		}
	}
}

func TestAnalyzeHeap_Empty(t *testing.T) {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "alloc_space", Unit: "bytes"},
		},
	}
	rep, err := AnalyzeHeap(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeHeap: %v", err)
	}
	if rep.TotalBytes != 0 {
		t.Errorf("expected 0 bytes, got %d", rep.TotalBytes)
	}
}
