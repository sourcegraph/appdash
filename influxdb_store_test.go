package appdash

import "testing"

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
		if got != c.Want {
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
	got := schemasFromAnnotations(anns)
	want := "HTTPClient,HTTPServer,name"
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}
