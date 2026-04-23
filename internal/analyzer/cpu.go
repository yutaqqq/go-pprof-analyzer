package analyzer

import (
	"sort"
	"strings"

	"github.com/google/pprof/profile"

	"github.com/yutaqqq/go-pprof-analyzer/internal/parser"
)

// CPUEntry represents a hot function in a CPU profile.
type CPUEntry struct {
	Function  string
	File      string
	FlatNs    int64
	CumulNs   int64
	FlatPct   float64
	CumulPct  float64
	Recommend string
}

// CPUReport is the full result of CPU analysis.
type CPUReport struct {
	TotalNs  int64
	HotPaths []CPUEntry
}

// AnalyzeCPU analyses a CPU profile and returns the top hot functions.
func AnalyzeCPU(p *profile.Profile, topN int) (*CPUReport, error) {
	idx := parser.ValueIndex(p, "cpu")
	if idx == 0 {
		idx = parser.ValueIndex(p, "samples")
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

	entries := make([]CPUEntry, 0, len(flat))
	for k, f := range flat {
		c := cumul[k]
		e := CPUEntry{
			Function: k.name,
			File:     k.file,
			FlatNs:   f,
			CumulNs:  c,
		}
		if total > 0 {
			e.FlatPct = float64(f) / float64(total) * 100
			e.CumulPct = float64(c) / float64(total) * 100
		}
		e.Recommend = cpuRecommend(k.name, e.FlatPct)
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].FlatNs > entries[j].FlatNs
	})
	if topN > 0 && len(entries) > topN {
		entries = entries[:topN]
	}

	return &CPUReport{TotalNs: total, HotPaths: entries}, nil
}

func cpuRecommend(fn string, pct float64) string {
	lower := strings.ToLower(fn)
	switch {
	case pct > 20:
		return "Функция занимает значительную долю CPU. Приоритетная цель для оптимизации."
	case strings.Contains(lower, "lock") || strings.Contains(lower, "mutex"):
		return "Lock-contention в горячем пути. Рассмотрите sharded mutex или lock-free структуры."
	case strings.Contains(lower, "gc") || strings.Contains(lower, "sweep") || strings.Contains(lower, "mallocgc"):
		return "GC-нагрузка. Сократите аллокации: sync.Pool, переиспользование буферов, arena."
	case strings.Contains(lower, "syscall") || strings.Contains(lower, "read") || strings.Contains(lower, "write"):
		return "Частые syscall. Используйте буферизацию (bufio) или пакетирование запросов."
	}
	return ""
}
