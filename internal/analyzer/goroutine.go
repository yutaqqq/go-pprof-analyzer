package analyzer

import (
	"sort"
	"strings"

	"github.com/google/pprof/profile"

	"github.com/yutaqqq/go-pprof-analyzer/internal/parser"
)

// GoroutineEntry represents a group of goroutines sharing the same stack.
type GoroutineEntry struct {
	TopFrame string
	Stack    []string
	Count    int
}

// LeakCandidate is a goroutine stack that grew between two profile snapshots.
type LeakCandidate struct {
	TopFrame string
	Stack    []string
	Before   int
	After    int
	Delta    int
}

// AnalyzeGoroutines returns goroutine groups sorted by count descending.
func AnalyzeGoroutines(p *profile.Profile) ([]GoroutineEntry, error) {
	idx, ok := parser.ValueIndex(p, "goroutine")
	if !ok {
		idx = 0
	}

	type stackKey string
	counts := make(map[stackKey]int)
	stacks := make(map[stackKey][]string)

	for _, s := range p.Sample {
		if idx >= len(s.Value) {
			continue
		}
		count := int(s.Value[idx])

		var frames []string
		for _, loc := range s.Location {
			for _, line := range loc.Line {
				if line.Function != nil {
					frames = append(frames, line.Function.Name)
				}
			}
		}
		key := stackKey(strings.Join(frames, "\n"))
		counts[key] += count
		if _, ok := stacks[key]; !ok {
			stacks[key] = frames
		}
	}

	entries := make([]GoroutineEntry, 0, len(counts))
	for key, count := range counts {
		frames := stacks[key]
		top := ""
		if len(frames) > 0 {
			top = frames[0]
		}
		entries = append(entries, GoroutineEntry{
			TopFrame: top,
			Stack:    frames,
			Count:    count,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	return entries, nil
}

// DetectLeaks compares two goroutine profiles taken at different times and
// returns stacks whose count grew by at least minDelta.
func DetectLeaks(before, after *profile.Profile, minDelta int) ([]LeakCandidate, error) {
	beforeEntries, err := AnalyzeGoroutines(before)
	if err != nil {
		return nil, err
	}
	afterEntries, err := AnalyzeGoroutines(after)
	if err != nil {
		return nil, err
	}

	beforeMap := make(map[string]int, len(beforeEntries))
	for _, e := range beforeEntries {
		beforeMap[strings.Join(e.Stack, "\n")] = e.Count
	}

	var candidates []LeakCandidate
	for _, e := range afterEntries {
		key := strings.Join(e.Stack, "\n")
		b := beforeMap[key]
		delta := e.Count - b
		if delta >= minDelta {
			candidates = append(candidates, LeakCandidate{
				TopFrame: e.TopFrame,
				Stack:    e.Stack,
				Before:   b,
				After:    e.Count,
				Delta:    delta,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Delta > candidates[j].Delta
	})
	return candidates, nil
}
