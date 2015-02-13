package appdash

import "testing"

func TestTrace_TreeString(t *testing.T) {
	t.Skip("TODO")

	x := &Trace{
		Span: Span{
			ID:          SpanID{1, 1, 0},
			Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
		},
		Sub: []*Trace{
			{
				Span: Span{
					ID:          SpanID{1, 2, 1},
					Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
				},
				Sub: []*Trace{
					{
						Span: Span{
							ID:          SpanID{1, 3, 2},
							Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
						},
					},
				},
			},
			{
				Span: Span{
					ID:          SpanID{1, 4, 1},
					Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
				},
				Sub: []*Trace{
					{
						Span: Span{
							ID:          SpanID{1, 5, 4},
							Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
						},
					},
					{
						Span: Span{
							ID:          SpanID{1, 6, 4},
							Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
						},
					},
				},
			},
		},
	}

	want := ``

	if ts := x.TreeString(); ts != want {
		t.Errorf("got TreeString\n%s\n\nwant TreeString\n%s", ts, want)
	}
}

func TestTrace_FindSpan(t *testing.T) {
	x := &Trace{
		Span: Span{
			ID:          SpanID{1, 1, 0},
			Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
		},
		Sub: []*Trace{
			{
				Span: Span{
					ID:          SpanID{1, 2, 1},
					Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
				},
				Sub: []*Trace{
					{
						Span: Span{
							ID:          SpanID{1, 3, 2},
							Annotations: []Annotation{{Key: "k", Value: []byte("v")}},
						},
					},
				},
			},
		},
	}

	testSpanIDs := []ID{1, 2, 3}
	for _, id := range testSpanIDs {
		span := x.FindSpan(id)
		if span == nil {
			t.Errorf("%v: got nil, want found", id)
		}
		if span.Span.ID.Span != id {
			t.Errorf("%v: got span ID %v, want %v", id, span.Span.ID.Span, id)
		}
	}
}
