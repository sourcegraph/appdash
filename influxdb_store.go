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
	defaultTracesPerPage  int    = 10             // Default number of traces per page.
	releaseDBName         string = "appdash"      // InfluxDB release DB name.
	schemasFieldName      string = "schemas"      // Span's measurement field name for schemas field.
	schemasFieldSeparator string = ","            // Span's measurement character separator for schemas field.
	spanMeasurementName   string = "spans"        // InfluxDB container name for trace spans.
	testDBName            string = "appdash_test" // InfluxDB test DB name (will be deleted entirely in test mode).
)

type mode int

const (
	releaseMode mode = iota // Default InfluxDBStore mode.
	testMode                // Used to setup InfluxDBStore for tests.
)

// Compile-time "implements" check.
var _ interface {
	Store
	Queryer
} = (*InfluxDBStore)(nil)

var (
	errMultipleSeries        = errors.New("unexpected multiple series")
	zeroID            string = ID(0).String() // zeroID is ID's zero value as string.
)

// pointFields -> influxDBClient.Point.Fields
type pointFields map[string]interface{}

type InfluxDBStore struct {
	adminUser InfluxDBAdminUser       // InfluxDB server auth credentials.
	con       *influxDBClient.Client  // InfluxDB client connection.
	dbName    string                  // InfluxDB database name for this store.
	defaultRP InfluxDBRetentionPolicy // Default retention policy for `dbName`.

	// When set to `testMode` - `testDBName` will be dropped and created, so newly database is ready for tests.
	mode          mode                   // Used to check current mode(release or test).
	server        *influxDBServer.Server // InfluxDB API server.
	tracesPerPage int                    // Number of traces per page.
}

func (in *InfluxDBStore) Collect(id SpanID, anns ...Annotation) error {
	// Find a span's point, if found it will be rewritten with new given annotations(`anns`)
	// if not found, a new span's point will be write to `in.dbName`.
	p, err := in.findSpanPoint(id)
	if err != nil {
		return err
	}

	// trace_id, span_id & parent_id are mostly used as part of the "where" part on queries so
	// to have performant queries these are set as tags(InfluxDB indexes tags).
	tags := map[string]string{
		"trace_id":  id.Trace.String(),
		"span_id":   id.Span.String(),
		"parent_id": id.Parent.String(),
	}

	// Annotations `anns` are set as fields(InfluxDB does not index fields).
	fields := make(map[string]interface{}, len(anns))
	for _, ann := range anns {
		fields[ann.Key] = string(ann.Value)
	}

	if p != nil { // span exists on `in.dbName`.
		p.Measurement = spanMeasurementName
		p.Tags = tags

		// Using extendFields & withoutEmptyFields in order to have pointFields that only contains:
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

		// `schemasFieldName` field contains all the schemas found on `anns`.
		// Eg. fields[schemasFieldName] = "HTTPClient,HTTPServer"
		fields[schemasFieldName] = schemasFromAnnotations(anns)
		p = &influxDBClient.Point{
			Measurement: spanMeasurementName,
			Tags:        tags,
			Fields:      fields,
			Time:        time.Now().UTC(),
		}
	}

	// A single point represents one span.
	pts := []influxDBClient.Point{*p}
	bps := influxDBClient.BatchPoints{
		Points:   pts,
		Database: in.dbName,
	}
	_, writeErr := in.con.Write(bps)
	if writeErr != nil {
		return writeErr
	}
	return nil
}

func (in *InfluxDBStore) Trace(id ID) (*Trace, error) {
	trace := &Trace{}
	q := fmt.Sprintf("SELECT * FROM spans WHERE trace_id='%s'", id)
	result, err := in.executeOneQuery(q)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}
	if len(result.Series) == 0 {
		return nil, ErrTraceNotFound
	}

	// Expecting only one element, which contains the queried spans information.
	if len(result.Series) > 1 {
		return nil, errMultipleSeries
	}

	var (
		rootSpanSet bool
		children    []*Trace
	)

	spans, err := spansFromRow(result.Series[0])
	if err != nil {
		return nil, err
	}

	// Iterate over spans to find and set `trace`'s root span & append children spans as sub-traces to `children` for later usage.
	for _, span := range spans {
		var isRootSpan bool
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
			children = append(children, &Trace{Span: *span})
		}
	}
	if err := addChildren(trace, children); err != nil {
		return nil, err
	}
	return trace, nil
}

func (in *InfluxDBStore) Traces(opts TracesOpts) ([]*Trace, error) {
	traces := make([]*Trace, 0)
	rootSpansQuery := fmt.Sprintf("SELECT * FROM spans WHERE parent_id='%s'", zeroID)
	if opts.Timespan != (Timespan{}) {
		start := opts.Timespan.S.UTC().Format(time.RFC3339Nano)
		end := opts.Timespan.E.UTC().Format(time.RFC3339Nano)
		rootSpansQuery += fmt.Sprintf(" AND time >= '%s' AND time <= '%s'", start, end)
	}
	rootSpansQuery += fmt.Sprintf(" LIMIT %d", in.tracesPerPage)
	rootSpansResult, err := in.executeOneQuery(rootSpansQuery)
	if err != nil {
		return nil, err
	}
	if rootSpansResult.Err != nil {
		return nil, rootSpansResult.Err
	}
	if len(rootSpansResult.Series) == 0 {
		return traces, nil
	}

	// Expecting only one element, which contains the queried spans information.
	if len(rootSpansResult.Series) > 1 {
		return nil, errMultipleSeries
	}

	// Cache to keep track of traces to be returned.
	tracesCache := make(map[ID]*Trace, 0)

	rootSpans, err := spansFromRow(rootSpansResult.Series[0])
	if err != nil {
		return nil, err
	}

	for _, span := range rootSpans {
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

	// Queries for all children spans of the root traces.
	childrenSpansQuery := fmt.Sprintf("SELECT * FROM spans %s", where)
	childrenSpansResult, err := in.executeOneQuery(childrenSpansQuery)
	if err != nil {
		return nil, err
	}

	// Cache to keep track all trace children of root traces to be returned.
	children := make(map[ID][]*Trace, 0) // Span.ID.Trace -> []*Trace

	childrenSpans, err := spansFromRow(childrenSpansResult.Series[0])
	if err != nil {
		return nil, err
	}

	// Iterates over `childrenSpans` to fill `children` cache.
	for _, span := range childrenSpans {
		trace, present := tracesCache[span.ID.Trace]
		if !present { // Root trace not added.
			return nil, errors.New("parent not found")
		} else { // Root trace already added, append `child` to `children` for later usage.
			child := &Trace{Span: *span}
			t, found := children[trace.ID.Trace]
			if !found {
				children[trace.ID.Trace] = []*Trace{child}
			} else {
				children[trace.ID.Trace] = append(t, child)
			}
		}
	}

	// Iterates over `tracesCache` to find and set their trace children.
	for _, trace := range tracesCache {
		traceChildren, present := children[trace.ID.Trace]
		if present {
			if err := addChildren(trace, traceChildren); err != nil {
				return nil, err
			}
		}
		traces = append(traces, trace)
	}
	return traces, nil
}

func (in *InfluxDBStore) Close() error {
	return in.server.Close()
}

func (in *InfluxDBStore) createDBIfNotExists() error {
	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", in.dbName)

	// If `in.defaultRP` info is provided, it's used to extend the query in order to create the database with
	// a default retention policy.
	if in.defaultRP.Duration != "" {
		q = fmt.Sprintf("%s WITH DURATION %s", q, in.defaultRP.Duration)

		// Retention policy name must be placed to the end of the query or it will be syntactically incorrect.
		if in.defaultRP.Name != "" {
			q = fmt.Sprintf("%s NAME %s", q, in.defaultRP.Name)
		}
	}

	// If there are no errors, query execution was successfully - either DB was created or already exists.
	response, err := in.con.Query(influxDBClient.Query{Command: q})
	if err != nil {
		return err
	}
	if err := response.Error(); err != nil {
		return err
	}
	return nil
}

// createAdminUserIfNotExists finds admin user(`in.adminUser`) if not found it's created.
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
	if !userInfo.Admin { // must be admin user.
		return errors.New("failed to validate InfluxDB user type, found non-admin user")
	}
	return nil
}

func (in *InfluxDBStore) executeOneQuery(command string) (*influxDBClient.Result, error) {
	response, err := in.con.Query(influxDBClient.Query{
		Command:  command,
		Database: in.dbName,
	})
	if err != nil {
		return nil, err
	}
	if err := response.Error(); err != nil {
		return nil, err
	}

	// Expecting one result, since a single query is executed.
	if len(response.Results) != 1 {
		return nil, errors.New("unexpected number of results for an influxdb single query")
	}
	return &response.Results[0], nil
}

func (in *InfluxDBStore) findSpanPoint(ID SpanID) (*influxDBClient.Point, error) {
	q := fmt.Sprintf(`SELECT * FROM spans WHERE trace_id='%s' AND span_id='%s' AND parent_id='%s'`, ID.Trace, ID.Span, ID.Parent)
	result, err := in.executeOneQuery(q)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}
	if len(result.Series) == 0 {
		return nil, nil
	}
	if len(result.Series) > 1 {
		return nil, errMultipleSeries
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

	// TODO: Upgrade to client v2, see: github.com/influxdata/influxdb/blob/master/client/v2/client.go
	// We're currently using v1.
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
	switch in.mode {
	case testMode:
		if err := in.setUpTestMode(); err != nil {
			return err
		}
	default:
		if err := in.setUpReleaseMode(); err != nil {
			return err
		}
	}
	if err := in.createDBIfNotExists(); err != nil {
		return err
	}

	// TODO: let lib users decide `in.tracesPerPage` through InfluxDBStoreConfig.
	in.tracesPerPage = defaultTracesPerPage
	return nil
}

func (in *InfluxDBStore) setUpReleaseMode() error {
	in.dbName = releaseDBName
	return nil
}

func (in *InfluxDBStore) setUpTestMode() error {
	in.dbName = testDBName
	response, err := in.con.Query(influxDBClient.Query{
		Command: fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName),
	})
	if err != nil {
		return err
	}
	if err := response.Error(); err != nil {
		return err
	}
	return nil
}

func annotationsFromEvents(a Annotations) (Annotations, error) {
	var (
		annotations Annotations
		events      []Event
	)
	if err := UnmarshalEvents(a, &events); err != nil {
		return nil, err
	}
	for _, e := range events {
		anns, err := MarshalEvent(e)
		if err != nil {
			return nil, err
		}
		annotations = append(annotations, anns...)
	}
	return annotations, nil
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

// fieldToSpanID converts given field to span ID.
func fieldToSpanID(field interface{}, errFieldType error) (*ID, error) {
	f, ok := field.(string)
	if !ok {
		return nil, errFieldType
	}
	id, err := ParseID(f)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// filterSchemas returns `Annotations` which contains items taken from `anns`.
// Some items from `anns` won't be included(those which were not saved by `InfluxDBStore.Collect(...)`).
func filterSchemas(anns []Annotation) Annotations {
	var annotations Annotations

	// Finds an annotation which: `Annotation.Key` is equal to `schemasFieldName`.
	schemasAnn := findSchemasAnnotation(anns)

	// Converts `schemasAnn.Value` into slice of strings, each item is a schema.
	// Eg. schemas := []string{"HTTPClient", "HTTPServer"}
	schemas := strings.Split(string(schemasAnn.Value), schemasFieldSeparator)

	// Iterates over `anns` to check if each annotation should be included or not to the `annotations` be returned.
	for _, a := range anns {
		if strings.HasPrefix(a.Key, schemaPrefix) { // Check if current annotation is schema related one.
			schema := a.Key[len(schemaPrefix):] // Excludes the schema prefix part.

			// Checks if `schema` exists in `schemas`, if so means current annotation was saved by `InfluxDBStore.Collect(...)`.
			// If does not exist it means current annotation is empty on `InfluxDBStore.dbName` but still included within a query result.
			// Eg. If point "f" with a field "foo" & point "b" with a field "bar" are written to the same InfluxDB measurement
			// and later queried, the result will include two fields: "foo" & "bar" for both points, even though each was written with one field.
			if schemaExists(schema, schemas) { // Saved by `InfluxDBStore.Collect(...)` so should be added.
				annotations = append(annotations, a)
			} else { // Do not add current annotation, is empty & not saved by `InfluxDBStore.Collect(...)`.
				continue
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

// findSchemasAnnotation finds & returns an annotation which: `Annotation.Key` is equal to `schemasFieldName`.
func findSchemasAnnotation(anns []Annotation) *Annotation {
	for _, a := range anns {
		if a.Key == schemasFieldName {
			return &a
		}
	}
	return nil
}

// findTraceParent walks through `rootTrace` to look for `child`; once found — it's trace parent is returned.
func findTraceParent(root, child *Trace) *Trace {
	var walkToParent func(root, child *Trace) *Trace
	walkToParent = func(root, child *Trace) *Trace {
		if root.ID.Span == child.ID.Parent {
			return root
		}
		for _, sub := range root.Sub {
			if sub.ID.Span == child.ID.Parent {
				return sub
			}
			if r := walkToParent(sub, child); r != nil {
				return r
			}
		}
		return nil
	}
	return walkToParent(root, child)
}

// mergeSchemasField merges new and old which are a set of schemas(strings) separated by `schemasFieldSeparator`.
// Returns the result of merging new & old without duplications.
func mergeSchemasField(new, old interface{}) (string, error) {
	// Since new and old have the same data structures(a set of strings separated by `schemasFieldSeparator`).
	// So same logic is applied to both.
	fields := []interface{}{new, old}
	var strFields []string

	// Iterates over fields to convert each into a string and appends it to `strFields` for later usage.
	for _, field := range fields {
		switch field.(type) {
		case string:
			strFields = append(strFields, field.(string))
		case nil:
			continue
		default:
			return "", fmt.Errorf("unexpected schema field type: %v", reflect.TypeOf(field))
		}
	}

	// Schemas cache, used to keep track schemas to be returned(without duplications).
	schemas := make(map[string]string, 0)

	// Iterates over `strFields` to convert each into a slice([]string), then iterates over it in order to
	// add each to `schemas` if not present already.
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

	// Returns a string which contains all the schemas separated by `schemasFieldSeparator`.
	return strings.Join(result, schemasFieldSeparator), nil
}

// schemasFromAnnotations returns a string(a set of schemas(strings) separated by `schemasFieldSeparator`) - eg. "HTTPClient,HTTPServer,name".
// Each schema is extracted from each `Annotation.Key` from `anns`.
func schemasFromAnnotations(anns []Annotation) string {
	var schemas []string
	for _, ann := range anns {

		// Checks if current annotation is schema related.
		if strings.HasPrefix(ann.Key, schemaPrefix) {
			schemas = append(schemas, ann.Key[len(schemaPrefix):])
		}
	}
	return strings.Join(schemas, schemasFieldSeparator)
}

// addChildren adds `children` to `root`; each child is appended to it's trace parent.
func addChildren(root *Trace, children []*Trace) error {
	var (
		addFn   func()                 // Handles children appending to it's trace parent.
		retries int    = len(children) // Maximum number of retries to add `children` elements to `root`.
		try     int                    // Current number of try to add `children` elements to `root`.
	)
	addFn = func() {
		for i := len(children) - 1; i >= 0; i-- {
			child := children[i]
			t := findTraceParent(root, child)
			if t != nil { // Trace found.
				if t.Sub == nil { // Empty sub-traces slice.
					t.Sub = []*Trace{child}
				} else { // Non-empty sub-traces slice.
					t.Sub = append(t.Sub, child)
				}

				// Removes current child since was added to it's parent.
				children = append(children[:i], children[i+1:]...)
			}
		}
	}

	// Loops until all `children` elements were added to it's trace parent or when maximum number of retries reached.
	for {
		if len(children) == 0 {
			break
		}

		// At this point, all children were added to their parent spans. Any children
		// left over in the children slice do not have parents. This could happen if,
		// for example, a parent service fails to record its span information to the
		// collection server but its downstream services do send their span information
		// properly. In this case, we gracefully degrade by adding these orphan spans to
		// the root span.
		if try == retries {

			// Iterates over children(without parent found on `root`) and appends them as sub-traces to `root`.
			for _, child := range children {
				if child.ID.Trace == root.ID.Trace {
					root.Sub = append(root.Sub, child)
				}
			}
			return nil
		}
		addFn()
		try++
	}
	return nil
}

// withoutEmptyFields filters `pf` and returns `pointFields` excluding those that have empty values.
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

// spansFromRow rebuilds spans from given InfluxDB row.
func spansFromRow(row influxDBModels.Row) ([]*Span, error) {
	var spans []*Span

	// Iterates through all `row.Values`, each represents a set of single span information(ids and annotations).
	for _, fields := range row.Values {
		span := &Span{
			Annotations: make(Annotations, 0),
		}

		// Iterates over fields, each field might be a span's annotation value or span's ID(either a TraceID, SpanID or ParentID).
		for i, field := range fields {
			var (
				// column is current field column's name (Eg. field ='Server.Request.Method', columns[i] = 'GET').
				column string = row.Columns[i]

				// errFieldType is an error for unexpected field type.
				errFieldType error = fmt.Errorf("unexpected field type: %v", reflect.TypeOf(field))
			)

			// Checks if current column is some span's ID, if so set to the span & continue with next field.
			switch column {
			case "trace_id":
				traceID, err := fieldToSpanID(field, errFieldType)
				if err != nil {
					return spans, err
				}
				span.ID.Trace = *traceID
				continue
			case "span_id":
				spanID, err := fieldToSpanID(field, errFieldType)
				if err != nil {
					return spans, err
				}
				span.ID.Span = *spanID
				continue
			case "parent_id":
				parentID, err := fieldToSpanID(field, errFieldType)
				if err != nil {
					return spans, err
				}
				span.ID.Parent = *parentID
				continue
			}

			// At this point the current field is a span's annotation value.
			var value []byte // Annotation value.
			switch field.(type) {
			case string:
				value = []byte(field.(string))
			case nil:
			default:
				return nil, errFieldType
			}
			span.Annotations = append(span.Annotations, Annotation{
				Key:   column,
				Value: value,
			})
		}
		anns, err := annotationsFromEvents(filterSchemas(span.Annotations))
		if err != nil {
			return nil, err
		}
		span.Annotations = anns
		spans = append(spans, span)
	}
	return spans, nil
}

type InfluxDBRetentionPolicy struct {
	Name     string // Name used to indentify this retention policy.
	Duration string // How long InfluxDB keeps the data. Eg: "1h", "1d", "1w".
}

type InfluxDBStoreConfig struct {
	AdminUser InfluxDBAdminUser
	BuildInfo *influxDBServer.BuildInfo
	DefaultRP InfluxDBRetentionPolicy
	Mode      mode
	Server    *influxDBServer.Config
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
	in := InfluxDBStore{
		adminUser: config.AdminUser,
		defaultRP: config.DefaultRP,
		mode:      config.Mode,
	}
	if err := in.init(s); err != nil {
		return nil, err
	}
	return &in, nil
}
