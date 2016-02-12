package appdash

import (
	"sort"
	"strings"
	"testing"
)

func TestMergeSchemasField(t *testing.T) {
	cases := []struct {
		NewField string
		OldField string
		Want     string
	}{
		{NewField: "", OldField: "", Want: ""},
		{NewField: "HTTPClient", OldField: "", Want: "HTTPClient"},
		{NewField: "", OldField: "name", Want: "name"},
		{NewField: "HTTPClient", OldField: "name", Want: "HTTPClient,name"},
		{NewField: "HTTPServer", OldField: "HTTPClient,name", Want: "HTTPServer,HTTPClient,name"},
	}
	for i, c := range cases {
		got, err := mergeSchemasField(c.NewField, c.OldField)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = sortSchemas(got)
		want := sortSchemas(c.Want)
		if got != want {
			t.Fatalf("case #%d - got: %v, want: %v", i, got, c.Want)
		}
	}
}

func TestSchemasFromAnnotations(t *testing.T) {
	anns := []Annotation{
		Annotation{Key: schemaPrefix + "HTTPClient"},
		Annotation{Key: schemaPrefix + "HTTPServer"},
		Annotation{Key: schemaPrefix + "name"},
	}
	got := sortSchemas(schemasFromAnnotations(anns))
	want := sortSchemas("HTTPClient,HTTPServer,name")
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

// sortSchemas sorts schemas(strings) within `s` which is
// a set of schemas separated by `schemasFieldSeparator`.
func sortSchemas(s string) string {
	schemas := strings.Split(s, schemasFieldSeparator)
	sort.Sort(bySchemaText(schemas))
	return strings.Join(schemas, schemasFieldSeparator)
}

type bySchemaText []string

func (bs bySchemaText) Len() int           { return len(bs) }
func (bs bySchemaText) Swap(i, j int)      { bs[i], bs[j] = bs[j], bs[i] }
func (bs bySchemaText) Less(i, j int) bool { return bs[i] < bs[j] }
