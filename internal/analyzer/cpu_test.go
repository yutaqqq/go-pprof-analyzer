package analyzer

import (
	"testing"

	"github.com/google/pprof/profile"
)

func makeCPUProfile(samples []struct {
	fn    string
	file  string
	ns    int64
}) *profile.Profile {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
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
			Value:    []int64{s.ns},
			Location: []*profile.Location{loc},
		})
	}
	return p
}

func TestAnalyzeCPU_TopN(t *testing.T) {
	p := makeCPUProfile([]struct {
		fn   string
		file string
		ns   int64
	}{
		{"pkg.HeavyFunc", "pkg/heavy.go", 800_000_000},
		{"pkg.LightFunc", "pkg/light.go", 50_000_000},
		{"pkg.MediumFunc", "pkg/medium.go", 150_000_000},
	})

	rep, err := AnalyzeCPU(p, 2)
	if err != nil {
		t.Fatalf("AnalyzeCPU: %v", err)
	}
	if len(rep.HotPaths) != 2 {
		t.Errorf("expected 2 hot paths, got %d", len(rep.HotPaths))
	}
	if rep.HotPaths[0].Function != "pkg.HeavyFunc" {
		t.Errorf("expected pkg.HeavyFunc at top, got %s", rep.HotPaths[0].Function)
	}
}

func TestAnalyzeCPU_Percentages(t *testing.T) {
	p := makeCPUProfile([]struct {
		fn   string
		file string
		ns   int64
	}{
		{"pkg.Hot", "pkg/hot.go", 700},
		{"pkg.Cold", "pkg/cold.go", 300},
	})

	rep, err := AnalyzeCPU(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeCPU: %v", err)
	}
	if rep.TotalNs != 1000 {
		t.Errorf("expected TotalNs=1000, got %d", rep.TotalNs)
	}
	for _, e := range rep.HotPaths {
		if e.Function == "pkg.Hot" && (e.FlatPct < 69 || e.FlatPct > 71) {
			t.Errorf("expected ~70%% for pkg.Hot, got %.1f%%", e.FlatPct)
		}
	}
}

func TestAnalyzeCPU_Recommendations(t *testing.T) {
	p := makeCPUProfile([]struct {
		fn   string
		file string
		ns   int64
	}{
		{"runtime.mallocgc", "runtime/malloc.go", 900_000_000},
		{"pkg.Other", "pkg/other.go", 100_000_000},
	})

	rep, err := AnalyzeCPU(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeCPU: %v", err)
	}
	for _, e := range rep.HotPaths {
		if e.Function == "runtime.mallocgc" && e.Recommend == "" {
			t.Error("expected GC recommendation for mallocgc, got empty")
		}
	}
}

func TestAnalyzeCPU_Empty(t *testing.T) {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "cpu", Unit: "nanoseconds"},
		},
	}
	rep, err := AnalyzeCPU(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeCPU: %v", err)
	}
	if rep.TotalNs != 0 {
		t.Errorf("expected TotalNs=0, got %d", rep.TotalNs)
	}
	if len(rep.HotPaths) != 0 {
		t.Errorf("expected no hot paths, got %d", len(rep.HotPaths))
	}
}

func TestAnalyzeCPU_FallbackToSamples(t *testing.T) {
	// Profile uses "samples" type instead of "cpu".
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
		},
	}
	fn := &profile.Function{ID: 1, Name: "pkg.Fn", Filename: "pkg/fn.go"}
	p.Function = append(p.Function, fn)
	loc := &profile.Location{ID: 1, Line: []profile.Line{{Function: fn}}}
	p.Location = append(p.Location, loc)
	p.Sample = append(p.Sample, &profile.Sample{Value: []int64{42}, Location: []*profile.Location{loc}})

	rep, err := AnalyzeCPU(p, 10)
	if err != nil {
		t.Fatalf("AnalyzeCPU: %v", err)
	}
	if rep.TotalNs != 42 {
		t.Errorf("expected TotalNs=42 from samples type, got %d", rep.TotalNs)
	}
}
