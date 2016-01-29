package appdash

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"time"

	influxDBClient "github.com/influxdb/influxdb/client"
	influxDBServer "github.com/influxdb/influxdb/cmd/influxd/run"
	influxDBModels "github.com/influxdb/influxdb/models"
)

const (
	dbName               string = "appdash" // InfluxDB db name.
	spanMeasurementName  string = "spans"   // InfluxDB container name for trace spans.
	defaultTracesPerPage int    = 10        // Default number of traces per page.
)

// Compile-time "implements" check.
var _ interface {
	Store
	Queryer
} = (*InfluxDBStore)(nil)

type InfluxDBStore struct {
	con           *influxDBClient.Client // InfluxDB client connection.
	server        *influxDBServer.Server // InfluxDB API server.
	tracesPerPage int                    // Number of traces per page.
}

func (in *InfluxDBStore) Collect(id SpanID, anns ...Annotation) error {
	// Current strategy is to remove existing span and save new one
	// instead of updating the existing one.
	// TODO: explore a more efficient alternative strategy.
	if err := in.removeSpanIfExists(id); err != nil {
		return err
	}
	// trace_id, span_id & parent_id are set as tags
	// because InfluxDB tags are indexed & those values
	// are uselater on queries.
	tags := make(map[string]string, 3)
	tags["trace_id"] = id.Trace.String()
	tags["span_id"] = id.Span.String()
	tags["parent_id"] = id.Parent.String()
	// Saving annotations as InfluxDB measurement spans fields
	// which are not indexed.
	fields := make(map[string]interface{}, len(anns))
	for _, ann := range anns {
		fields[ann.Key] = string(ann.Value)
	}
	// InfluxDB point represents a single span.
	pts := []influxDBClient.Point{
		influxDBClient.Point{
			Measurement: spanMeasurementName,
			Tags:        tags,   // indexed metadata.
			Fields:      fields, // non-indexed metadata.
			Time:        time.Now(),
			Precision:   "s",
		},
	}
	bps := influxDBClient.BatchPoints{
		Points:          pts,
		Database:        dbName,
		RetentionPolicy: "default",
	}
	_, err := in.con.Write(bps)
	if err != nil {
		return err
	}
	return nil
}

func (in *InfluxDBStore) Trace(id ID) (*Trace, error) {
	trace := &Trace{}
	// GROUP BY * -> meaning group by all tags(trace_id, span_id & parent_id)
	// grouping by all tags includes those and it's values on the query response.
	q := fmt.Sprintf("SELECT * FROM spans WHERE trace_id='%s' GROUP BY *", id)
	result, err := in.executeOneQuery(q)
	if err != nil {
		return nil, err
	}
	// result.Series -> A slice containing all the spans.
	if len(result.Series) == 0 {
		return nil, errors.New("trace not found")
	}
	var isRootSpan bool
	// Iterate over series(spans) to create trace children's & set trace fields.
	for _, s := range result.Series {
		span, err := newSpanFromRow(&s)
		if err != nil {
			return nil, err
		}
		if span.ID.Parent == 0 && isRootSpan {
			// Must be a single root span.
			return nil, errors.New("unexpected multiple root spans")
		}
		if span.ID.Parent == 0 && !isRootSpan {
			isRootSpan = true
		}
		annotations, err := annotationsFromRow(&s)
		if err != nil {
			return trace, nil
		}
		span.Annotations = *annotations
		if isRootSpan { // root span.
			trace.Span = *span
		} else { // children span.
			trace.Sub = append(trace.Sub, &Trace{Span: *span})
		}
	}
	return trace, nil
}

func (in *InfluxDBStore) Traces() ([]*Trace, error) {
	traces := make([]*Trace, 0)
	// GROUP BY * -> meaning group by all tags(trace_id, span_id & parent_id)
	// grouping by all tags includes those and it's values on the query response.
	q := fmt.Sprintf("SELECT * FROM spans GROUP BY * LIMIT %d", in.tracesPerPage)
	result, err := in.executeOneQuery(q)
	if err != nil {
		return nil, err
	}
	// result.Series -> A slice containing all the spans.
	if len(result.Series) == 0 {
		return traces, nil
	}
	// Cache to keep track of traces to be returned.
	tracesCache := make(map[ID]*Trace, 0)
	// Iterate over series(spans) to create traces.
	for _, s := range result.Series {
		var isRootSpan bool
		span, err := newSpanFromRow(&s)
		if err != nil {
			return nil, err
		}
		annotations, err := annotationsFromRow(&s)
		if err != nil {
			return nil, err
		}
		if span.ID.Parent == 0 {
			isRootSpan = true
		}
		span.Annotations = *annotations
		if isRootSpan { // root span.
			trace, present := tracesCache[span.ID.Trace]
			if !present {
				tracesCache[span.ID.Trace] = &Trace{Span: *span}
			} else { // trace already added just update the span.
				trace.Span = *span
			}
		} else { // children span.
			trace, present := tracesCache[span.ID.Trace]
			if !present { // root trace not added yet.
				tracesCache[span.ID.Trace] = &Trace{Sub: []*Trace{&Trace{Span: *span}}}
			} else { // root trace already added so append a sub trace.
				trace.Sub = append(trace.Sub, &Trace{Span: *span})
			}
		}
	}
	for _, t := range tracesCache {
		traces = append(traces, t)
	}
	return traces, nil
}

func (in *InfluxDBStore) Close() {
	in.server.Close()
}

func (in *InfluxDBStore) createDBIfNotExists() error {
	// If no errors query execution was successfully - either DB was created or already exists.
	response, err := in.con.Query(influxDBClient.Query{
		Command: fmt.Sprintf("%s %s", "CREATE DATABASE IF NOT EXISTS", dbName),
	})
	if err != nil {
		return err
	}
	if response.Error() != nil {
		return response.Error()
	}
	return nil
}

func (in *InfluxDBStore) executeOneQuery(command string) (*influxDBClient.Result, error) {
	response, err := in.con.Query(influxDBClient.Query{
		Command:  command,
		Database: dbName,
	})
	if err != nil {
		return nil, err
	}
	if response.Error() != nil {
		return nil, response.Error()
	}
	// Expecting one result, since a single query is executed.
	if len(response.Results) != 1 {
		return nil, errors.New("unexpected number of results for an influxdb single query")
	}
	return &response.Results[0], nil
}

func (in *InfluxDBStore) init(server *influxDBServer.Server) error {
	in.server = server
	url, err := url.Parse(fmt.Sprintf("http://%s:%d", influxDBClient.DefaultHost, influxDBClient.DefaultPort))
	if err != nil {
		return err
	}
	con, err := influxDBClient.NewClient(influxDBClient.Config{URL: *url})
	if err != nil {
		return err
	}
	in.con = con
	if err := in.createDBIfNotExists(); err != nil {
		return err
	}
	in.tracesPerPage = defaultTracesPerPage
	return nil
}

func (in *InfluxDBStore) removeSpanIfExists(id SpanID) error {
	cmd := fmt.Sprintf(`
		DROP SERIES FROM spans WHERE trace_id = '%s' AND span_id = '%s' AND parent_id = '%s'
	`, id.Trace.String(), id.Span.String(), id.Parent.String())
	_, err := in.executeOneQuery(cmd)
	if err != nil {
		return err
	}
	return nil
}

func annotationsFromRow(r *influxDBModels.Row) (*Annotations, error) {
	// Actually an influxDBModels.Row represents a single InfluxDB serie.
	// r.Values[n] is a slice containing span's annotation values.
	var fields []interface{}
	if len(r.Values) == 1 {
		fields = r.Values[0]
	}
	// len(r.Values) might be greater than one - meaning there are
	// some spans to drop, see: InfluxDBStore.Collect(...).
	// If so last one is picked.
	if len(r.Values) > 1 {
		fields = r.Values[len(r.Values)-1]
	}
	annotations := make(Annotations, len(fields))
	// Iterates over fields which represent span's annotation values.
	for i, field := range fields {
		// It is safe to do column[0] (eg. 'Server.Request.Method')
		// matches fields[0] (eg. 'GET')
		key := r.Columns[i]
		var value []byte
		switch field.(type) {
		case string:
			value = []byte(field.(string))
		case nil:
		default:
			return nil, fmt.Errorf("unexpected field type: %v", reflect.TypeOf(field))
		}
		a := Annotation{
			Key:   key,
			Value: value,
		}
		annotations = append(annotations, a)
	}
	return &annotations, nil
}

func newSpanFromRow(r *influxDBModels.Row) (*Span, error) {
	span := &Span{}
	traceID, err := ParseID(r.Tags["trace_id"])
	if err != nil {
		return nil, err
	}
	spanID, err := ParseID(r.Tags["span_id"])
	if err != nil {
		return nil, err
	}
	parentID, err := ParseID(r.Tags["parent_id"])
	if err != nil {
		return nil, err
	}
	span.ID = SpanID{
		Trace:  ID(traceID),
		Span:   ID(spanID),
		Parent: ID(parentID),
	}
	return span, nil
}

func NewInfluxDBStore(c *influxDBServer.Config, bi *influxDBServer.BuildInfo) (*InfluxDBStore, error) {
	//TODO: add Authentication.
	s, err := influxDBServer.NewServer(c, bi)
	if err != nil {
		return nil, err
	}
	if err := s.Open(); err != nil {
		return nil, err
	}
	var in InfluxDBStore
	if err := in.init(s); err != nil {
		return nil, err
	}
	return &in, nil
}
