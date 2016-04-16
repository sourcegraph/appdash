package traceapp

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cznic/mathutil"
	"sourcegraph.com/sourcegraph/appdash"
)

// dashboardRow represents a single row in the dashboard. It is encoded to JSON.
type dashboardRow struct {
	Name                      string
	Average, Min, Max, StdDev time.Duration
	Timespans                 int
	URL                       string
}

// newDashboardRow returns a new dashboard row with it's items calculated from
// the given aggregation event and timespan events (the returned row represents
// the whole aggregation event).
//
// The returned row does not have the URL field set.
func newDashboardRow(a appdash.AggregateEvent, timespans []appdash.TimespanEvent) dashboardRow {
	row := dashboardRow{
		Name:      a.Name,
		Timespans: len(timespans),
	}

	// Calculate sum and mean (row.Average), while determining min/max.
	sum := big.NewInt(0)
	for _, ts := range timespans {
		d := ts.End().Sub(ts.Start())
		sum.Add(sum, big.NewInt(int64(d)))
		if row.Min == 0 || d < row.Min {
			row.Min = d
		}
		if row.Max == 0 || d > row.Max {
			row.Max = d
		}
	}
	avg := big.NewInt(0).Div(sum, big.NewInt(int64(len(timespans))))
	row.Average = time.Duration(avg.Int64())

	// Calculate std. deviation.
	sqDiffSum := big.NewInt(0)
	for _, ts := range timespans {
		d := ts.End().Sub(ts.Start())
		diff := big.NewInt(0).Sub(big.NewInt(int64(d)), avg)
		sqDiffSum.Add(sqDiffSum, diff.Mul(diff, diff))
	}
	stdDev := big.NewInt(0).Div(sqDiffSum, big.NewInt(int64(len(timespans))))
	stdDev = mathutil.SqrtBig(stdDev)
	row.StdDev = time.Duration(stdDev.Int64())

	// TODO(slimsag): if we can make the table display the data as formatted by
	// Go (row.Average.String), we'll get much nicer display. But it means we'll
	// need to perform custom sorting on the table (it will think "9ms" > "1m",
	// for example).

	// Divide into milliseconds.
	row.Average = row.Average / time.Millisecond
	row.Min = row.Min / time.Millisecond
	row.Max = row.Max / time.Millisecond
	row.StdDev = row.StdDev / time.Millisecond
	return row
}

// serverDashboard serves the dashboard page.
func (a *App) serveDashboard(w http.ResponseWriter, r *http.Request) error {
	uData, err := a.Router.URLTo(DashboardDataRoute)
	if err != nil {
		return err
	}

	return a.renderTemplate(w, r, "dashboard.html", http.StatusOK, &struct {
		TemplateCommon
		DataURL string
	}{
		DataURL: uData.String(),
	})
}

// serveDashboardData serves the JSON data requested by the dashboards table.
func (a *App) serveDashboardData(w http.ResponseWriter, r *http.Request) error {
	// Parse the query for the start & end timeline durations.
	var (
		query      = r.URL.Query()
		start, end time.Time
	)
	basis := time.Now().Add(-72 * time.Hour)
	if s := query.Get("start"); len(s) > 0 {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		// e.g. if (v)start==0, it'll be -72hrs ago
		start = basis.Add(time.Duration(v) * time.Hour)
	}
	if s := query.Get("end"); len(s) > 0 {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		// .eg. if (v)end==72, it'll be time.Now()
		end = basis.Add(time.Duration(v) * time.Hour)
	}

	traces, err := a.Queryer.Traces(appdash.TracesOpts{
		Timespan: appdash.Timespan{S: start, E: end},
	})
	if err != nil {
		return err
	}

	// Grab the URL to the traces page.
	tracesURL, err := a.Router.URLTo(TracesRoute)
	if err != nil {
		return err
	}

	// Important: If it is a nil slice it will be encoded to JSON as null, and the
	// bootstrap-table library will not update the table with "no entries".
	rows, err := influxDBStoreRows(traces, tracesURL)
	if err != nil {
		return err
	}

	// Encode to JSON.
	j, err := json.Marshal(rows)
	if err != nil {
		return err
	}

	// Write out.
	_, err = io.Copy(w, bytes.NewReader(j))
	return err
}

// influxDBStoreRows groups given traces by span name which are used to create a slice of `dashboardRow` to be returned.
func influxDBStoreRows(traces []*appdash.Trace, tracesURL *url.URL) ([]dashboardRow, error) {
	rows := make([]dashboardRow, 0)                         // Dashboard rows to be returned.
	groupBySpanName := make(map[string][]*appdash.Trace, 0) // Traces grouped by span name, Trace.Span.Name() -> Trace.

	// Iterate over given traces to group them by span name.
	for _, trace := range traces {
		spanName := trace.Span.Name()
		if t, found := groupBySpanName[spanName]; found {
			groupBySpanName[spanName] = append(t, trace)
		} else {
			groupBySpanName[spanName] = []*appdash.Trace{trace}
		}
	}

	// Iterates over grouped traces to create a dashboardRow and append it to the slice to be returned.
	for spanName, traces := range groupBySpanName {
		aggregateEvent := appdash.AggregateEvent{Name: spanName}
		timespans := []appdash.TimespanEvent{}

		// Iterate over traces in order to populate `aggregateEvent` & `timespans`.
		for _, trace := range traces {
			aggregateEvent.Slowest = append(aggregateEvent.Slowest, trace.ID.Span)
			timespan, err := trace.TimespanEvent()
			if err != nil {
				return rows, err
			}
			timespans = append(timespans, timespan)
		}
		tracesURL.RawQuery = aggregateEvent.SlowestRawQuery()

		// Create the row of data.
		row := newDashboardRow(aggregateEvent, timespans)
		row.URL = tracesURL.String()
		rows = append(rows, row)
	}
	return rows, nil
}
