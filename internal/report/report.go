package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yutaqqq/go-pprof-analyzer/internal/analyzer"
	"github.com/yutaqqq/go-pprof-analyzer/internal/diff"
)

// Data is the full analysis result passed to report writers.
type Data struct {
	GeneratedAt time.Time
	ProfilePath string

	Heap       *analyzer.HeapReport
	CPU        *analyzer.CPUReport
	Goroutines []analyzer.GoroutineEntry
	Leaks      []analyzer.LeakCandidate
	Diff       *diff.Report
}

// WriteMarkdown writes a Markdown report to w.
func WriteMarkdown(d *Data, w io.Writer) error {
	p := func(format string, args ...any) {
		fmt.Fprintf(w, format+"\n", args...)
	}

	p("# pprof Analysis Report")
	p("")
	p("**Файл:** `%s`  ", d.ProfilePath)
	p("**Сгенерирован:** %s", d.GeneratedAt.Format("2006-01-02 15:04:05"))
	p("")

	if d.Heap != nil {
		p("---")
		p("")
		p("## Heap: топ аллокаторов")
		p("")
		p("**Тип профиля:** %s  ", d.Heap.ProfileType)
		p("**Всего байт:** %s", fmtBytes(d.Heap.TotalBytes))
		p("")
		p("| Функция | Файл | Flat | Flat%% | Cumul | Cumul%% | Рекомендация |")
		p("|---------|------|------|-------|-------|--------|--------------|")
		for _, e := range d.Heap.TopAllocators {
			p("| `%s` | `%s` | %s | %.1f%% | %s | %.1f%% | %s |",
				shortName(e.Function), shortFile(e.File),
				fmtBytes(e.FlatBytes), e.FlatPct,
				fmtBytes(e.CumulBytes), e.CumulPct,
				e.Recommend)
		}
		p("")
		if len(d.Heap.PoolCandidates) > 0 {
			p("### Кандидаты для sync.Pool")
			p("")
			for _, e := range d.Heap.PoolCandidates {
				p("- `%s` — %.1f%% от общего объёма", shortName(e.Function), e.FlatPct)
			}
			p("")
		}
	}

	if d.CPU != nil {
		p("---")
		p("")
		p("## CPU: горячие пути")
		p("")
		p("**Всего CPU-времени:** %s", fmtNs(d.CPU.TotalNs))
		p("")
		p("| Функция | Файл | Flat | Flat%% | Cumul | Cumul%% | Рекомендация |")
		p("|---------|------|------|-------|-------|--------|--------------|")
		for _, e := range d.CPU.HotPaths {
			p("| `%s` | `%s` | %s | %.1f%% | %s | %.1f%% | %s |",
				shortName(e.Function), shortFile(e.File),
				fmtNs(e.FlatNs), e.FlatPct,
				fmtNs(e.CumulNs), e.CumulPct,
				e.Recommend)
		}
		p("")
	}

	if len(d.Goroutines) > 0 {
		p("---")
		p("")
		p("## Goroutines: топ стеков")
		p("")
		p("| Количество | Верхний фрейм |")
		p("|-----------|---------------|")
		for _, g := range d.Goroutines {
			p("| %d | `%s` |", g.Count, shortName(g.TopFrame))
		}
		p("")
	}

	if len(d.Leaks) > 0 {
		p("---")
		p("")
		p("## Возможные утечки горутин")
		p("")
		p("> Стеки, чей счётчик вырос между снимками.")
		p("")
		for i, l := range d.Leaks {
			p("### Кандидат %d: `%s`", i+1, shortName(l.TopFrame))
			p("")
			p("- **До:** %d  ", l.Before)
			p("- **После:** %d  ", l.After)
			p("- **Прирост:** +%d", l.Delta)
			p("")
			p("```")
			for _, frame := range l.Stack {
				p("  %s", frame)
			}
			p("```")
			p("")
		}
	}

	if d.Diff != nil {
		p("---")
		p("")
		p("## Diff: до / после")
		p("")
		p("**Тип значений:** `%s` (%s)", d.Diff.ValueType, d.Diff.Unit)
		p("")
		if len(d.Diff.Increased) > 0 {
			p("### Выросло")
			p("")
			p("| Функция | До | После | Δ | Δ%% |")
			p("|---------|-----|-------|---|-----|")
			for _, e := range d.Diff.Increased {
				p("| `%s` | %d | %d | +%d | +%.1f%% |",
					shortName(e.Function), e.Before, e.After, e.Delta, e.DeltaPct)
			}
			p("")
		}
		if len(d.Diff.Decreased) > 0 {
			p("### Уменьшилось")
			p("")
			p("| Функция | До | После | Δ | Δ%% |")
			p("|---------|-----|-------|---|-----|")
			for _, e := range d.Diff.Decreased {
				p("| `%s` | %d | %d | %d | %.1f%% |",
					shortName(e.Function), e.Before, e.After, e.Delta, e.DeltaPct)
			}
			p("")
		}
		if len(d.Diff.New) > 0 {
			p("### Новые аллокаторы")
			p("")
			for _, e := range d.Diff.New {
				p("- `%s`: %d %s", shortName(e.Function), e.After, d.Diff.Unit)
			}
			p("")
		}
	}

	return nil
}

// WriteJSON writes the report as JSON to w.
func WriteJSON(d *Data, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

func fmtBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func fmtNs(ns int64) string {
	switch {
	case ns >= 1e9:
		return fmt.Sprintf("%.2fs", float64(ns)/1e9)
	case ns >= 1e6:
		return fmt.Sprintf("%.2fms", float64(ns)/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.2fµs", float64(ns)/1e3)
	default:
		return fmt.Sprintf("%dns", ns)
	}
}

func shortName(fn string) string {
	if idx := strings.LastIndex(fn, "/"); idx >= 0 {
		return fn[idx+1:]
	}
	return fn
}

func shortFile(file string) string {
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		return file[idx+1:]
	}
	return file
}
