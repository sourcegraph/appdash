package appdash

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	influxDBClient "github.com/influxdata/influxdb/client"
	influxDBServer "github.com/influxdata/influxdb/cmd/influxd/run"
	influxDBModels "github.com/influxdata/influxdb/models"
	influxDBErrors "github.com/influxdata/influxdb/services/meta"
)

const (
	dbName                string = "appdash" // InfluxDB db name.
	defaultTracesPerPage  int    = 10        // Default number of traces per page.
	schemasFieldName      string = "schemas" // Span's measurement field name for schemas field.
	schemasFieldSeparator string = ","       // Span's measurement character separator for schemas field.
	spanMeasurementName   string = "spans"   // InfluxDB container name for trace spans.
)

// Compile-time "implements" check.
var _ interface {
	Store
	Queryer
} = (*InfluxDBStore)(nil)

// zeroID is ID's zero value as string.
var zeroID string = ID(0).String()

// pointFields -> influxDBClient.Point.Fields
type pointFields map[string]interface{}

type InfluxDBStore struct {
	adminUser     InfluxDBAdminUser      // InfluxDB server auth credentials.
	con           *influxDBClient.Client // InfluxDB client connection.
	server        *influxDBServer.Server // InfluxDB API server.
	tracesPerPage int                    // Number of traces per page.
}

func (in *InfluxDBStore) Collect(id SpanID, anns ...Annotation) error {
	// Find a span's point, if found it will be rewritten with new annotations(`anns`)
	// if not found, a new span's point will be created.
	p, err := in.findSpanPoint(id)
	if err != nil {
		return err
	}

	// trace_id, span_id & parent_id are set as tags
	// because InfluxDB tags are indexed & those values
	// are used later on queries.
	tags := map[string]string{
		"trace_id":  id.Trace.String(),
		"span_id":   id.Span.String(),
		"parent_id": id.Parent.String(),
	}

	// Saving annotations as InfluxDB measurement spans fields
	// which are not indexed.
	fields := make(map[string]interface{}, len(anns))
	for _, ann := range anns {
		fields[ann.Key] = string(ann.Value)
	}

	if p != nil { // span exists on DB.
		p.Measurement = spanMeasurementName
		p.Tags = tags

		// Using extendFields & withoutEmptyFields in order to have
		// pointFields that only contains:
		// - Fields that are not saved on DB.
		// - Fields that are saved but have empty values.
		fields := extendFields(fields, withoutEmptyFields(p.Fields))
		schemas, err := mergeSchemasField(schemasFromAnnotations(anns), p.Fields[schemasFieldName])
		if err != nil {
			return err
		}

		// `schemas` contains the result of merging(without duplications)
		// schemas already saved on DB and schemas present on `anns`.
		fields[schemasFieldName] = schemas
		p.Fields = fields
	} else { // new span to be saved on DB.

		// A field `schemasFieldName` contains all the schemas found on `anns`.
		// Eg. fields[schemasFieldName] = "HTTPClient,HTTPServer"
		fields[schemasFieldName] = schemasFromAnnotations(anns)
		p = &influxDBClient.Point{
			Measurement: spanMeasurementName,
			Tags:        tags,   // indexed metadata.
			Fields:      fields, // non-indexed metadata.
			Time:        time.Now().UTC(),
		}
	}

	// InfluxDB point represents a single span.
	pts := []influxDBClient.Point{*p}
	bps := influxDBClient.BatchPoints{
		Points:          pts,
		Database:        dbName,
		RetentionPolicy: "default",
	}
	_, writeErr := in.con.Write(bps)
	if writeErr != nil {
		return writeErr
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

	// Iterate over series(spans) to create trace children's & set trace fields.
	var rootSpanSet bool
	for _, s := range result.Series {
		var isRootSpan bool
		span, err := newSpanFromRow(&s)
		if err != nil {
			return nil, err
		}
		annotations, err := annotationsFromRow(&s)
		if err != nil {
			return trace, nil
		}
		span.Annotations = filterSchemas(*annotations)
		if span.ID.IsRoot() && rootSpanSet {
			return nil, errors.New("unexpected multiple root spans")
		}
		if span.ID.IsRoot() && !rootSpanSet {
			isRootSpan = true
		}
		if isRootSpan { // root span.
			trace.Span = *span
			rootSpanSet = true
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
	rootSpansQuery := fmt.Sprintf("SELECT * FROM spans WHERE parent_id='%s' GROUP BY * LIMIT %d", zeroID, in.tracesPerPage)
	rootSpansResult, err := in.executeOneQuery(rootSpansQuery)
	if err != nil {
		return nil, err
	}

	// result.Series -> A slice containing all the spans.
	if len(rootSpansResult.Series) == 0 {
		return traces, nil
	}

	// Cache to keep track of traces to be returned.
	tracesCache := make(map[ID]*Trace, 0)

	// Iterate over series(spans) to create traces.
	for _, s := range rootSpansResult.Series {
		span, err := newSpanFromRow(&s)
		if err != nil {
			return nil, err
		}
		annotations, err := annotationsFromRow(&s)
		if err != nil {
			return nil, err
		}
		span.Annotations = *annotations
		_, present := tracesCache[span.ID.Trace]
		if !present {
			tracesCache[span.ID.Trace] = &Trace{Span: *span}
		} else {
			return nil, errors.New("duplicated root span")
		}
	}

	// Using 'OR' since 'IN' not supported yet.
	where := `WHERE `
	var i int = 1
	for _, trace := range tracesCache {
		where += fmt.Sprintf("(trace_id='%s' AND parent_id!='%s')", trace.Span.ID.Trace, zeroID)

		// Adds 'OR' except for last iteration.
		if i != len(tracesCache) && len(tracesCache) > 1 {
			where += " OR "
		}
		i += 1
	}

	// Queries for all children spans of the traces to be returned.
	childrenSpansQuery := fmt.Sprintf("SELECT * FROM spans %s GROUP BY *", where)
	childrenSpansResult, err := in.executeOneQuery(childrenSpansQuery)
	if err != nil {
		return nil, err
	}

	// Iterate over series(children spans) to create sub-traces
	// and associates sub-traces with it's parent trace.
	for _, s := range childrenSpansResult.Series {
		span, err := newSpanFromRow(&s)
		if err != nil {
			return nil, err
		}
		annotations, err := annotationsFromRow(&s)
		if err != nil {
			return nil, err
		}
		span.Annotations = filterSchemas(*annotations)
		trace, present := tracesCache[span.ID.Trace]
		if !present { // Root trace not added.
			return nil, errors.New("parent not found")
		} else { // Root trace already added so append a sub-trace.
			trace.Sub = append(trace.Sub, &Trace{Span: *span})
		}
	}
	for _, trace := range tracesCache {
		traces = append(traces, trace)
	}
	return traces, nil
}

func (in *InfluxDBStore) Close() error {
	return in.server.Close()
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

// createAdminUserIfNotExists creates an admin user
// using `in.adminUser` credentials if does not exist.
func (in *InfluxDBStore) createAdminUserIfNotExists() error {
	userInfo, err := in.server.MetaClient.Authenticate(in.adminUser.Username, in.adminUser.Password)
	if err == influxDBErrors.ErrUserNotFound {
		if _, createUserErr := in.server.MetaClient.CreateUser(in.adminUser.Username, in.adminUser.Password, true); createUserErr != nil {
			return createUserErr
		}
		return nil
	} else {
		return err
	}
	if !userInfo.Admin {
		return errors.New("failed to validate InfluxDB user type, found non-admin user")
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

func (in *InfluxDBStore) findSpanPoint(ID SpanID) (*influxDBClient.Point, error) {
	q := fmt.Sprintf(`
		SELECT * FROM spans WHERE trace_id='%s' AND span_id='%s' AND parent_id='%s' GROUP BY *
	`, ID.Trace, ID.Span, ID.Parent)
	result, err := in.executeOneQuery(q)
	if err != nil {
		return nil, err
	}
	if len(result.Series) == 0 {
		return nil, nil
	}
	if len(result.Series) > 1 {
		return nil, errors.New("unexpected multiple series")
	}
	r := result.Series[0]
	if len(r.Values) == 0 {
		return nil, errors.New("unexpected empty series")
	}
	p := influxDBClient.Point{
		Fields: make(pointFields, 0),
	}
	fields := r.Values[0]
	for i, field := range fields {
		key := r.Columns[i]
		switch field.(type) {
		case string:
			// time field is set by InfluxDB not related to annotations.
			if key == "time" {
				t, err := time.Parse(time.RFC3339Nano, field.(string))
				if err != nil {
					return nil, err
				}
				p.Time = t
			}
			p.Fields[key] = field.(string)
		case nil:
			continue
		default:
			return nil, fmt.Errorf("unexpected field type: %v", reflect.TypeOf(field))
		}
	}
	return &p, err
}

func (in *InfluxDBStore) init(server *influxDBServer.Server) error {
	in.server = server
	url, err := url.Parse(fmt.Sprintf("http://%s:%d", influxDBClient.DefaultHost, influxDBClient.DefaultPort))
	if err != nil {
		return err
	}
	con, err := influxDBClient.NewClient(influxDBClient.Config{
		URL:      *url,
		Username: in.adminUser.Username,
		Password: in.adminUser.Password,
	})
	if err != nil {
		return err
	}
	in.con = con
	if err := in.createAdminUserIfNotExists(); err != nil {
		return err
	}
	if err := in.createDBIfNotExists(); err != nil {
		return err
	}
	// TODO: support specifying the number of traces per page.
	in.tracesPerPage = defaultTracesPerPage
	return nil
}

func annotationsFromRow(r *influxDBModels.Row) (*Annotations, error) {
	// Actually an influxDBModels.Row represents a single InfluxDB serie.
	// r.Values[n] is a slice containing span's annotation values.
	var fields []interface{}
	if len(r.Values) == 1 {
		fields = r.Values[0]
	}

	// len(r.Values) cannot be greater than 1.
	// Values[0] is the slice containing a span's
	// annotation values.
	if len(r.Values) > 1 {
		return nil, errors.New("unexpected multiple row values")
	}
	annotations := make(Annotations, 0)

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

// extendFields replaces existing items on dst from src.
func extendFields(dst, src pointFields) pointFields {
	for k, v := range src {
		if _, present := dst[k]; present {
			dst[k] = v
		}
	}
	return dst
}

// filterSchemas returns `Annotations` with items taken from `anns`.
// It finds the annotation with key: `schemaFieldName`, which is later use
// to discard schema related annotations not present on it's value.
func filterSchemas(anns []Annotation) Annotations {
	var annotations Annotations

	// Finds the annotation with key `schemasFieldName`.
	schemasAnn := findSchemasAnnotation(anns)

	// Convert it to a string slice which contains the schemas.
	// Eg. schemas := []string{"HTTPClient", "HTTPServer"}
	schemas := strings.Split(string(schemasAnn.Value), schemasFieldSeparator)

	// Iterate over `anns` to check if each annotation is a schema related one
	// if so it's added to the `annotations` be returned, but only if it's present
	// on `schemas`.
	// If annotation is not schema related, it's added to annotations returned.
	for _, a := range anns {
		if strings.HasPrefix(a.Key, schemaPrefix) {
			schema := a.Key[len(schemaPrefix):]

			// If schema does not exists; annotation `a` is not added to
			// the `annotations` be returned because it was not saved
			// by `Collect(...)`.
			// But exists because InfluxDB returns all fields(annotations)
			// even those ones not explicit written by `Collect(...)`.
			//
			// Eg. if point "a" is written with a field "foo" &
			// point "b" with a field "bar" (both "a" & "b" written in the
			// same  measurement), when querying for those points the result
			// will contain two fields "foo" & "bar", even though field "bar"
			// was not present when writing Point "a".
			if schemaExists(schema, schemas) {
				// Schema exists, meaning `Collect(...)` method
				// saved this annotation.
				annotations = append(annotations, a)
			}
		} else {
			// Not a schema related annotation so just add it.
			annotations = append(annotations, a)
		}
	}
	return annotations
}

// schemaExists checks if `schema` is present on `schemas`.
func schemaExists(schema string, schemas []string) bool {
	for _, s := range schemas {
		if schema == s {
			return true
		}
	}
	return false
}

// findSchemasAnnotation finds & returns an annotation
// with key: `schemasFieldName`.
func findSchemasAnnotation(anns []Annotation) *Annotation {
	for _, a := range anns {
		if a.Key == schemasFieldName {
			return &a
		}
	}
	return nil
}

// mergeSchemasField merges new and old which are a set of schemas(strings)
// separated by `schemasFieldSeparator` - eg. "HTTPClient,HTTPServer"
// Returns the result of merging new & old without duplications.
func mergeSchemasField(new, old interface{}) (string, error) {
	// Since both new and old are same data structures
	// (a set of strings separated by `schemasFieldSeparator`)
	// same code logic is applied.
	fields := []interface{}{new, old}
	var strFields []string

	// Iterate over fields in order to cast each to string type
	// and append it to `strFields` for later usage.
	for _, field := range fields {
		switch field.(type) {
		case string:
			strFields = append(strFields, field.(string))
		case nil:
			continue
		default:
			return "", fmt.Errorf("unexpected event field type: %v", reflect.TypeOf(field))
		}
	}

	// Cache for schemas; used to keep track of non duplicated schemas
	// to be returned.
	schemas := make(map[string]string, 0)

	// Iterate over `strFields` to transform each to a slice([]string)
	// which each element is an schema that are added to schemas cache.
	for _, strField := range strFields {
		if strField == "" {
			continue
		}
		sf := strings.Split(strField, schemasFieldSeparator)
		for _, s := range sf {
			if _, found := schemas[s]; !found {
				schemas[s] = s
			}
		}
	}

	var result []string
	for k, _ := range schemas {
		result = append(result, k)
	}

	// Return a string which contains all the schemas separated by `schemasFieldSeparator`.
	return strings.Join(result, schemasFieldSeparator), nil
}

// schemasFromAnnotations finds schemas in `anns` and builds a data structure
// which is a set of all schemas found, those are separated by `schemasFieldSeparator`
// and returned as string.
func schemasFromAnnotations(anns []Annotation) string {
	var schemas []string
	for _, ann := range anns {
		if strings.HasPrefix(ann.Key, schemaPrefix) { // Check if is an annotation for a schema.
			schemas = append(schemas, ann.Key[len(schemaPrefix):])
		}
	}
	return strings.Join(schemas, schemasFieldSeparator)
}

// withoutEmptyFields returns a pointFields without
// those fields that has empty values.
func withoutEmptyFields(pf pointFields) pointFields {
	r := make(pointFields, 0)
	for k, v := range pf {
		switch v.(type) {
		case string:
			if v.(string) == "" {
				continue
			}
			r[k] = v
		case nil:
			continue
		default:
			r[k] = v
		}
	}
	return r
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

type InfluxDBStoreConfig struct {
	BuildInfo *influxDBServer.BuildInfo
	Server    *influxDBServer.Config
	AdminUser InfluxDBAdminUser
}

type InfluxDBAdminUser struct {
	Username string
	Password string
}

func NewInfluxDBStore(config InfluxDBStoreConfig) (*InfluxDBStore, error) {
	s, err := influxDBServer.NewServer(config.Server, config.BuildInfo)
	if err != nil {
		return nil, err
	}
	if err := s.Open(); err != nil {
		return nil, err
	}
	in := InfluxDBStore{adminUser: config.AdminUser}
	if err := in.init(s); err != nil {
		return nil, err
	}
	return &in, nil
}
