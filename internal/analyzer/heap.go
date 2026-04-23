package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/pprof/profile"

	"github.com/yutaqqq/go-pprof-analyzer/internal/parser"
)

// HeapEntry represents a top memory-allocating function.
type HeapEntry struct {
	Function   string
	File       string
	FlatBytes  int64
	CumulBytes int64
	FlatPct    float64
	CumulPct   float64
	Recommend  string
}

// HeapReport is the full result of heap analysis.
type HeapReport struct {
	ProfileType    string // "alloc" or "inuse"
	TotalBytes     int64
	TopAllocators  []HeapEntry
	PoolCandidates []HeapEntry
}

// AnalyzeHeap analyses a heap profile and returns the top allocators with recommendations.
func AnalyzeHeap(p *profile.Profile, topN int) (*HeapReport, error) {
	// Prefer alloc_space for finding leak candidates; fall back to inuse_space.
	profileType := "alloc"
	idx, ok := parser.ValueIndex(p, "alloc_space")
	if !ok {
		idx, ok = parser.ValueIndex(p, "inuse_space")
		profileType = "inuse"
	}
	if !ok {
		return nil, fmt.Errorf("profile contains neither alloc_space nor inuse_space sample type")
	}

	type funcKey struct{ name, file string }
	flat := make(map[funcKey]int64)
	cumul := make(map[funcKey]int64)

	for _, s := range p.Sample {
		if idx >= len(s.Value) {
			continue
		}
		v := s.Value[idx]
		seen := make(map[funcKey]bool)
		for i, loc := range s.Location {
			for _, line := range loc.Line {
				if line.Function == nil {
					continue
				}
				key := funcKey{name: line.Function.Name, file: line.Function.Filename}
				if i == 0 {
					flat[key] += v
				}
				if !seen[key] {
					cumul[key] += v
					seen[key] = true
				}
			}
		}
	}

	var total int64
	for _, v := range flat {
		total += v
	}

	entries := make([]HeapEntry, 0, len(flat))
	for k, f := range flat {
		c := cumul[k]
		e := HeapEntry{
			Function:   k.name,
			File:       k.file,
			FlatBytes:  f,
			CumulBytes: c,
		}
		if total > 0 {
			e.FlatPct = float64(f) / float64(total) * 100
			e.CumulPct = float64(c) / float64(total) * 100
		}
		e.Recommend = heapRecommend(k.name, f, c, total)
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].FlatBytes > entries[j].FlatBytes
	})
	if topN > 0 && len(entries) > topN {
		entries = entries[:topN]
	}

	var poolCandidates []HeapEntry
	for _, e := range entries {
		if e.FlatPct > 5 && isPoolCandidate(e.Function) {
			poolCandidates = append(poolCandidates, e)
		}
	}

	return &HeapReport{
		ProfileType:    profileType,
		TotalBytes:     total,
		TopAllocators:  entries,
		PoolCandidates: poolCandidates,
	}, nil
}

func heapRecommend(fn string, flat, cumul, total int64) string {
	if total == 0 {
		return ""
	}
	pct := float64(flat) / float64(total) * 100

	lower := strings.ToLower(fn)

	switch {
	case pct > 30:
		return fmt.Sprintf("Доминирующий аллокатор (%.1f%% от общего объёма). Рассмотрите sync.Pool или arena-аллокатор.", pct)
	case strings.Contains(lower, "json") || strings.Contains(lower, "marshal") || strings.Contains(lower, "unmarshal"):
		return "JSON-аллокации в горячем пути. Используйте прямой маппинг вместо промежуточных структур; рассмотрите easyjson или sonic."
	case strings.Contains(lower, "fmt.") || strings.Contains(lower, "sprintf"):
		return "fmt.Sprintf аллоцирует на каждый вызов. Замените на strings.Builder или заранее выделенный буфер."
	case strings.Contains(lower, "append") || strings.Contains(lower, "growslice"):
		return "Рост slice. Инициализируйте с нужной ёмкостью: make([]T, 0, n)."
	case pct > 5 && isPoolCandidate(fn):
		return fmt.Sprintf("Часто создаваемый объект (%.1f%%). Рассмотрите sync.Pool.", pct)
	case flat > 0 && cumul > flat*3:
		return "Высокое кумулятивное значение относительно flat — проверьте транзитивные аллокации в цепочке вызовов."
	}
	return ""
}

func isPoolCandidate(fn string) bool {
	lower := strings.ToLower(fn)
	for _, kw := range []string{"new", "make", "create", "alloc", "get", "acquire"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
