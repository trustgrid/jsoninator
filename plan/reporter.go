package plan

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type reportType string

const (
	reportType_Change reportType = "change"
	reportType_Skip   reportType = "skip"
)

type change struct {
	name   string
	before any
	after  any
}

var reports = make(chan Reporter)

type Reporter struct {
	name    string
	changes []change
	skipped string
}

func NewReporter(name string) *Reporter {
	return &Reporter{
		name: name,
	}
}

func WithReporter(ctx context.Context, name string) (context.Context, func()) {
	r := NewReporter(name)
	return context.WithValue(ctx, "reporter", r), r.Close
}

func (r *Reporter) Skip(filter string) {
	r.skipped = filter
}

func (r *Reporter) Change(name string, before any, after any) {
	r.changes = append(r.changes, change{
		name:   name,
		before: before,
		after:  after,
	})
}

func (r *Reporter) Close() {
	reports <- *r
}

func closeReporters() {
	close(reports)
}

func report(ctx context.Context) {
	stamp := fmt.Sprintf("%s-%d", time.Now().Format("20060102-150405"), rand.Int())
	dir := filepath.Join("reports", stamp)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("unable to create reports directory", "err", err)
		panic(err)
	}

	fmt.Println("reports will be written to", dir)

	mkfile := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			slog.Error("unable to create report file", "name", name, "dir", dir, "err", err)
			panic(err)
		}
		return f
	}

	filterFile := mkfile("filtered.csv")
	changeFile := mkfile("changes.csv")
	noopFile := mkfile("noops.csv")
	defer filterFile.Close()
	defer changeFile.Close()
	defer noopFile.Close()

	filterCSV := csv.NewWriter(filterFile)
	changeCSV := csv.NewWriter(changeFile)
	noopCSV := csv.NewWriter(noopFile)
	defer filterCSV.Flush()
	defer changeCSV.Flush()
	defer noopCSV.Flush()

	filterCSV.Write([]string{"name", "filter"})
	changeCSV.Write([]string{"name", "field", "before", "after"})
	noopCSV.Write([]string{"name"})

	writeCSV := func(w *csv.Writer, record []string) {
		if err := w.Write(record); err != nil {
			slog.Error("unable to write report record", "record", record, "err", err)
		}
	}

	for {
		select {
		case r, ok := <-reports:
			if !ok {
				return
			}
			switch {
			case r.skipped != "":
				writeCSV(filterCSV, []string{r.name, r.skipped})
			case r.changes != nil:
				for _, c := range r.changes {
					writeCSV(changeCSV, []string{r.name, c.name, fmt.Sprintf("%v", c.before), fmt.Sprintf("%v", c.after)})
				}
			default:
				writeCSV(noopCSV, []string{r.name})
			}
		case <-ctx.Done():
			slog.Info("reporter exiting")
			return

		}
	}
}
