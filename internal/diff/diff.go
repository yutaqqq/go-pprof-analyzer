package diff

import (
	"fmt"
	"sort"

	"github.com/google/pprof/profile"

	"github.com/yutaqqq/go-pprof-analyzer/internal/parser"
)

// Entry is a function whose value changed between two profiles.
type Entry struct {
	Function string
	File     string
	Before   int64
	After    int64
	Delta    int64
	DeltaPct float64
}

// Report is the full diff between two profiles.
type Report struct {
	ValueType string
	Unit      string
	Increased []Entry
	Decreased []Entry
	New       []Entry // present in after, absent in before
	Gone      []Entry // present in before, absent in after
}

// Compare produces a diff of flat values between before and after profiles.
func Compare(before, after *profile.Profile, topN int) (*Report, error) {
	// Find a common value type.
	vType, unit, err := commonValueType(before, after)
	if err != nil {
		return nil, err
	}

	beforeIdx, _ := parser.ValueIndex(before, vType)
	afterIdx, _ := parser.ValueIndex(after, vType)
	beforeFlat := flatValues(before, beforeIdx)
	afterFlat := flatValues(after, afterIdx)

	all := make(map[funcKey]struct{})
	for k := range beforeFlat {
		all[k] = struct{}{}
	}
	for k := range afterFlat {
		all[k] = struct{}{}
	}

	r := &Report{ValueType: vType, Unit: unit}

	for k := range all {
		b := beforeFlat[k]
		a := afterFlat[k]
		delta := a - b

		e := Entry{
			Function: k.name,
			File:     k.file,
			Before:   b,
			After:    a,
			Delta:    delta,
		}
		if b > 0 {
			e.DeltaPct = float64(delta) / float64(b) * 100
		}

		switch {
		case b == 0:
			r.New = append(r.New, e)
		case a == 0:
			r.Gone = append(r.Gone, e)
		case delta > 0:
			r.Increased = append(r.Increased, e)
		case delta < 0:
			r.Decreased = append(r.Decreased, e)
		}
	}

	sortDesc := func(s []Entry) {
		sort.Slice(s, func(i, j int) bool { return abs64(s[i].Delta) > abs64(s[j].Delta) })
	}
	sortDesc(r.Increased)
	sortDesc(r.Decreased)
	sortDesc(r.New)
	sortDesc(r.Gone)

	if topN > 0 {
		r.Increased = clip(r.Increased, topN)
		r.Decreased = clip(r.Decreased, topN)
		r.New = clip(r.New, topN)
		r.Gone = clip(r.Gone, topN)
	}

	return r, nil
}

type funcKey struct{ name, file string }

func flatValues(p *profile.Profile, idx int) map[funcKey]int64 {
	m := make(map[funcKey]int64)
	for _, s := range p.Sample {
		if idx >= len(s.Value) {
			continue
		}
		v := s.Value[idx]
		if len(s.Location) == 0 {
			continue
		}
		for _, line := range s.Location[0].Line {
			if line.Function == nil {
				continue
			}
			k := funcKey{name: line.Function.Name, file: line.Function.Filename}
			m[k] += v
		}
	}
	return m
}

func commonValueType(a, b *profile.Profile) (string, string, error) {
	aTypes := make(map[string]string)
	for _, st := range a.SampleType {
		aTypes[st.Type] = st.Unit
	}
	for _, st := range b.SampleType {
		if unit, ok := aTypes[st.Type]; ok {
			return st.Type, unit, nil
		}
	}
	if len(a.SampleType) > 0 {
		return a.SampleType[0].Type, a.SampleType[0].Unit, nil
	}
	return "", "", fmt.Errorf("profiles have no common sample type")
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func clip[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
