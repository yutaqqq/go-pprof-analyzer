package analyzer

import (
	"testing"

	"github.com/google/pprof/profile"
)

func makeGoroutineProfile(stacks []struct {
	frames []string
	count  int
}) *profile.Profile {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "goroutine", Unit: "count"},
		},
	}
	for _, s := range stacks {
		var locs []*profile.Location
		for _, frame := range s.frames {
			fn := &profile.Function{ID: uint64(len(p.Function)) + 1, Name: frame}
			p.Function = append(p.Function, fn)
			loc := &profile.Location{
				ID:   uint64(len(p.Location)) + 1,
				Line: []profile.Line{{Function: fn}},
			}
			p.Location = append(p.Location, loc)
			locs = append(locs, loc)
		}
		p.Sample = append(p.Sample, &profile.Sample{
			Value:    []int64{int64(s.count)},
			Location: locs,
		})
	}
	return p
}

func TestAnalyzeGoroutines_Sorted(t *testing.T) {
	p := makeGoroutineProfile([]struct {
		frames []string
		count  int
	}{
		{[]string{"net/http.(*conn).serve"}, 50},
		{[]string{"database/sql.(*DB).query"}, 200},
		{[]string{"time.Sleep"}, 5},
	})

	entries, err := AnalyzeGoroutines(p)
	if err != nil {
		t.Fatalf("AnalyzeGoroutines: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Count != 200 {
		t.Errorf("expected top count=200, got %d", entries[0].Count)
	}
}

func TestDetectLeaks_Delta(t *testing.T) {
	before := makeGoroutineProfile([]struct {
		frames []string
		count  int
	}{
		{[]string{"net/http.(*conn).serve"}, 10},
		{[]string{"time.Sleep"}, 2},
	})

	after := makeGoroutineProfile([]struct {
		frames []string
		count  int
	}{
		{[]string{"net/http.(*conn).serve"}, 10}, // unchanged
		{[]string{"time.Sleep"}, 100},            // grew by 98
		{[]string{"myapp.leakyHandler"}, 50},     // new
	})

	leaks, err := DetectLeaks(before, after, 5)
	if err != nil {
		t.Fatalf("DetectLeaks: %v", err)
	}
	if len(leaks) == 0 {
		t.Fatal("expected leak candidates, got none")
	}
	// time.Sleep grew by 98
	found := false
	for _, l := range leaks {
		if l.TopFrame == "time.Sleep" && l.Delta == 98 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected time.Sleep delta=98, got: %v", leaks)
	}
}

func TestDetectLeaks_NoneUnderThreshold(t *testing.T) {
	before := makeGoroutineProfile([]struct {
		frames []string
		count  int
	}{
		{[]string{"worker.run"}, 10},
	})
	after := makeGoroutineProfile([]struct {
		frames []string
		count  int
	}{
		{[]string{"worker.run"}, 12}, // delta=2, below threshold of 5
	})

	leaks, err := DetectLeaks(before, after, 5)
	if err != nil {
		t.Fatalf("DetectLeaks: %v", err)
	}
	if len(leaks) != 0 {
		t.Errorf("expected no leaks below threshold, got %d", len(leaks))
	}
}
