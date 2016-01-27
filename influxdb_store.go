package appdash

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"time"

	influxDBClient "github.com/influxdb/influxdb/client"
	influxDBServer "github.com/influxdb/influxdb/cmd/influxd/run"
)

const (
	dbName              string = "appdash" // InfluxDB db name.
	spanMeasurementName string = "spans"   // InfluxDB container name for trace spans.
)

// Compile-time "implements" check.
var _ interface {
	Store
	Queryer
} = (*InfluxDBStore)(nil)

type InfluxDBStore struct {
	con    *influxDBClient.Client // InfluxDB client connection.
	server *influxDBServer.Server
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
			Tags:        tags,   //indexed metadata
			Fields:      fields, //non-indexed metadata
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
	t := &Trace{}
	// GROUP BY * -> meaning group by all tags(trace_id, span_id & parent_id)
	// grouping by all tags includes those and it's values on the query response.
	q := influxDBClient.Query{
		Command:  fmt.Sprintf("SELECT * FROM spans WHERE trace_id='%s' GROUP BY *", id),
		Database: dbName,
	}
	response, err := in.con.Query(q)
	if err != nil {
		return nil, err
	}
	if response.Error() != nil {
		return nil, response.Error()
	}
	// Expecting one result, since a single query is executed:
	// "SELECT * FROM spans ...".
	if len(response.Results) != 1 {
		return nil, errors.New("unexpected number of influxdb query response result")
	}
	// Slice series contains all the spans.
	if len(response.Results[0].Series) == 0 {
		return nil, errors.New("trace not found")
	}
	var isRootSpan bool
	// Iterate over series(spans) to create & set trace fields.
	for _, s := range response.Results[0].Series {
		traceID, err := ParseID(s.Tags["trace_id"])
		if err != nil {
			return nil, err
		}
		spanID, err := ParseID(s.Tags["span_id"])
		if err != nil {
			return nil, err
		}
		parentID, err := ParseID(s.Tags["parent_id"])
		if err != nil {
			return nil, err
		}
		if parentID == 0 && isRootSpan {
			// Must be a single root span.
			return nil, errors.New("unexpected multiple root spans")
		}
		if parentID == 0 && !isRootSpan {
			isRootSpan = true
		}
		span := Span{
			ID: SpanID{
				Trace:  ID(traceID),
				Span:   ID(spanID),
				Parent: ID(parentID),
			},
		}
		// s.Values[n] is a slice of span's annotation values
		// len(s.Values) might be greater than one - meaning there are
		// some to drop, see: InfluxDBStore.Collect(...).
		// if so last one is use.
		var fields []interface{}
		if len(s.Values) == 1 {
			fields = s.Values[0]
		}
		if len(s.Values) > 1 {
			fields = s.Values[len(s.Values)-1]
		}
		annotations := make(Annotations, len(fields))
		// Iterates over span's annotation values.
		for i, field := range fields {
			// It is safe to do column[0] (eg. 'Server.Request.Method')
			// matches fields[0] (eg. 'GET')
			key := s.Columns[i]
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
		span.Annotations = annotations
		if isRootSpan {
			t.Span = span
		} else { // children
			t.Sub = append(t.Sub, &Trace{Span: span})
		}
	}
	return t, nil
}

func (in *InfluxDBStore) Traces() ([]*Trace, error) {
	//TODO: implementation
	return nil, nil
}

func (in *InfluxDBStore) Close() {
	in.server.Close()
}

func (in *InfluxDBStore) createDBIfNotExists() error {
	v := url.Values{}
	v.Set("q", fmt.Sprintf("%s %s", "CREATE DATABASE IF NOT EXISTS ", dbName))
	url, err := url.Parse(fmt.Sprintf("%s/%s?%s", in.con.Addr(), "query", v.Encode()))
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if c := resp.StatusCode; c < 200 || c > 299 {
		defer resp.Body.Close()
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to create appdash database, response body: %s", string(b))
	}
	return nil
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
	return nil
}

func (in *InfluxDBStore) removeSpanIfExists(id SpanID) error {
	cmd := fmt.Sprintf(`
		DROP SERIES FROM spans WHERE trace_id = '%s' AND span_id = '%s' AND parent_id = '%s'
	`, id.Trace.String(), id.Span.String(), id.Parent.String())
	q := influxDBClient.Query{
		Command:  cmd,
		Database: dbName,
	}
	_, err := in.con.Query(q)
	if err != nil {
		return err
	}
	return nil
}

func NewInfluxDBStore(c *influxDBServer.Config, bi *influxDBServer.BuildInfo) (*InfluxDBStore, error) {
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
