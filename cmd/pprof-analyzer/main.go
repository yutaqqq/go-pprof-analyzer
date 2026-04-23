package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/yutaqqq/go-pprof-analyzer/internal/analyzer"
	"github.com/yutaqqq/go-pprof-analyzer/internal/diff"
	"github.com/yutaqqq/go-pprof-analyzer/internal/parser"
	"github.com/yutaqqq/go-pprof-analyzer/internal/report"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pprof-analyzer",
		Short: "Автоматический анализ pprof-профилей с рекомендациями",
	}
	root.AddCommand(analyzeCmd(), diffCmd(), leakCmd())
	return root
}

// ── analyze ──────────────────────────────────────────────────────────────────

func analyzeCmd() *cobra.Command {
	var (
		topN   int
		output string
		format string
	)

	cmd := &cobra.Command{
		Use:   "analyze <profile.pb.gz>",
		Short: "Анализ одного профиля (heap / cpu / goroutine)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			profilePath := args[0]

			p, err := parser.Load(profilePath)
			if err != nil {
				return err
			}

			d := &report.Data{
				GeneratedAt: time.Now(),
				ProfilePath: profilePath,
			}

			switch parser.DetectType(p) {
			case "heap":
				d.Heap, err = analyzer.AnalyzeHeap(p, topN)
			case "cpu":
				d.CPU, err = analyzer.AnalyzeCPU(p, topN)
			case "goroutine":
				d.Goroutines, err = analyzer.AnalyzeGoroutines(p)
			default:
				d.Heap, err = analyzer.AnalyzeHeap(p, topN)
			}
			if err != nil {
				return fmt.Errorf("analyze: %w", err)
			}

			return writeReport(d, output, format)
		},
	}

	cmd.Flags().IntVarP(&topN, "top", "n", 20, "число топ-записей в отчёте")
	cmd.Flags().StringVarP(&output, "output", "o", "", "файл для записи отчёта (по умолчанию stdout)")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "формат отчёта: markdown или json")
	return cmd
}

// ── diff ─────────────────────────────────────────────────────────────────────

func diffCmd() *cobra.Command {
	var (
		topN   int
		output string
		format string
	)

	cmd := &cobra.Command{
		Use:   "diff <before.pb.gz> <after.pb.gz>",
		Short: "Сравнение двух профилей до/после оптимизации",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			before, err := parser.Load(args[0])
			if err != nil {
				return fmt.Errorf("before: %w", err)
			}
			after, err := parser.Load(args[1])
			if err != nil {
				return fmt.Errorf("after: %w", err)
			}

			diffReport, err := diff.Compare(before, after, topN)
			if err != nil {
				return fmt.Errorf("diff: %w", err)
			}

			d := &report.Data{
				GeneratedAt: time.Now(),
				ProfilePath: args[0] + " → " + args[1],
				Diff:        diffReport,
			}
			return writeReport(d, output, format)
		},
	}

	cmd.Flags().IntVarP(&topN, "top", "n", 20, "число топ-записей в каждой секции")
	cmd.Flags().StringVarP(&output, "output", "o", "", "файл для записи отчёта")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "формат: markdown или json")
	return cmd
}

// ── leak ─────────────────────────────────────────────────────────────────────

func leakCmd() *cobra.Command {
	var (
		minDelta int
		output   string
		format   string
	)

	cmd := &cobra.Command{
		Use:   "leak <goroutine1.pb.gz> <goroutine2.pb.gz>",
		Short: "Детектор утечек горутин: сравнивает два снимка во времени",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			before, err := parser.Load(args[0])
			if err != nil {
				return fmt.Errorf("snapshot-1: %w", err)
			}
			after, err := parser.Load(args[1])
			if err != nil {
				return fmt.Errorf("snapshot-2: %w", err)
			}

			leaks, err := analyzer.DetectLeaks(before, after, minDelta)
			if err != nil {
				return fmt.Errorf("leak detection: %w", err)
			}

			if len(leaks) == 0 {
				fmt.Println("Утечек горутин не обнаружено.")
				return nil
			}

			d := &report.Data{
				GeneratedAt: time.Now(),
				ProfilePath: args[0] + " → " + args[1],
				Leaks:       leaks,
			}
			return writeReport(d, output, format)
		},
	}

	cmd.Flags().IntVarP(&minDelta, "min-delta", "d", 5, "минимальный прирост горутин для попадания в отчёт")
	cmd.Flags().StringVarP(&output, "output", "o", "", "файл для записи отчёта")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "формат: markdown или json")
	return cmd
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeReport(d *report.Data, output, format string) error {
	w := os.Stdout
	if output != "" {
		f, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	switch format {
	case "json":
		return report.WriteJSON(d, w)
	default:
		return report.WriteMarkdown(d, w)
	}
}
