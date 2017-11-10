package appdash

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestNewRootSpanID(t *testing.T) {
	id := NewRootSpanID()
	if id.Parent != 0 {
		t.Errorf("unexpected parent: %+v", id)
	}
	if id.Span == 0 {
		t.Errorf("zero Span: %+v", id)
	}
	if id.Trace == 0 {
		t.Errorf("zero root: %+v", id)
	}
	if id.Trace == id.Span {
		t.Errorf("duplicate IDs: %+v", id)
	}
}

func TestNewSpanID(t *testing.T) {
	root := NewRootSpanID()
	id := NewSpanID(root)
	if id.Parent != root.Span {
		t.Errorf("unexpected parent: %+v", id)
	}
	if id.Span == 0 {
		t.Errorf("zero Span: %+v", id)
	}
	if id.Trace != root.Trace {
		t.Errorf("mismatched root: %+v", id)
	}
}

func TestSpanIDString(t *testing.T) {
	id := SpanID{
		Trace: 100,
		Span:  300,
	}
	got := id.String()
	want := "0000000000000064/000000000000012c"
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSpanIDStringWithParent(t *testing.T) {
	id := SpanID{
		Trace:  100,
		Parent: 200,
		Span:   300,
	}
	actual := id.String()
	expected := "0000000000000064/000000000000012c/00000000000000c8"
	if actual != expected {
		t.Errorf("Was %#v, but expected %#v", actual, expected)
	}
}

func TestSpanIDFormat(t *testing.T) {
	id := SpanID{
		Trace: 100,
		Span:  300,
	}
	got := id.Format("/* %s */ %s", "SELECT 1")
	want := "/* 0000000000000064/000000000000012c */ SELECT 1"
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func ExampleSpanID_Format() {
	// Assume we're connected to a database.
	var (
		event  SpanID
		db     *sql.DB
		userID int
	)
	// This passes the root ID and the parent event ID to the database, which
	// allows us to correlate, for example, slow queries with the web requests
	// which caused them.
	query := event.Format(`/* %s/%s */ %s`, `SELECT email FROM users WHERE id = ?`)
	r := db.QueryRow(query, userID)
	if r == nil {
		panic("user not found")
	}
	var email string
	if err := r.Scan(&email); err != nil {
		panic("couldn't read email")
	}
	fmt.Printf("User's email: %s\n", email)
}

func TestParseSpanID(t *testing.T) {
	id, err := ParseSpanID("0000000000000064/000000000000012c")
	if err != nil {
		t.Fatal(err)
	}
	if id.Trace != 100 || id.Span != 300 {
		t.Errorf("unexpected ID: %+v", id)
	}
}

func TestParseSpanIDWithParent(t *testing.T) {
	id, err := ParseSpanID("0000000000000064/000000000000012c/0000000000000096")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Trace != 100 || id.Parent != 150 || id.Span != 300 {
		t.Errorf("unexpected event ID: %+v", id)
	}
}

func TestParseSpanIDMalformed(t *testing.T) {
	id, err := ParseSpanID(`0000000000000064000000000000012c`)
	if id != nil {
		t.Errorf("unexpected ID: %+v", id)
	}
	if err != ErrBadSpanID {
		t.Error(err)
	}
}

func TestParseSpanIDBadTrace(t *testing.T) {
	id, err := ParseSpanID("0000000000g000064/000000000000012c")
	if id != nil {
		t.Errorf("unexpected ID: %+v", id)
	}
	if err != ErrBadSpanID {
		t.Error(err)
	}
}

func TestParseSpanIDBadID(t *testing.T) {
	id, err := ParseSpanID("0000000000000064/0000000000g00012c")
	if id != nil {
		t.Errorf("unexpected ID: %+v", id)
	}
	if err != ErrBadSpanID {
		t.Error(err)
	}
}

func TestParseSpanIDBadParent(t *testing.T) {
	id, err := ParseSpanID("0000000000000064/000000000000012c/00000000000g0096")
	if id != nil {
		t.Errorf("unexpected event ID: %+v", id)
	}
	if err != ErrBadSpanID {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSpan_Name(t *testing.T) {
	namedSpan := &Span{Annotations: Annotations{{Key: "Name", Value: []byte("foo")}}}
	if want := "foo"; namedSpan.Name() != want {
		t.Errorf("got Name %q, want %q", namedSpan.Name(), want)
	}

	noNameSpan := &Span{}
	if want := ""; noNameSpan.Name() != want {
		t.Errorf("got Name %q, want %q", noNameSpan.Name(), want)
	}
}

type annotations Annotations

func (a annotations) Len() int           { return len(a) }
func (a annotations) Less(i, j int) bool { return a[i].Key < a[j].Key }
func (a annotations) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func BenchmarkNewRootSpanID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewRootSpanID()
	}
}

func BenchmarkNewSpanID(b *testing.B) {
	root := NewRootSpanID()
	for i := 0; i < b.N; i++ {
		NewSpanID(root)
	}
}

func BenchmarkSpanIDString(b *testing.B) {
	id := SpanID{
		Trace:  100,
		Parent: 200,
		Span:   300,
	}
	for i := 0; i < b.N; i++ {
		_ = id.String()
	}
}

func BenchmarkParseSpanID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ParseSpanID("0000000000000064/000000000000012c")
		if err != nil {
			b.Fatal(err)
		}
	}
}
