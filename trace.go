package appdash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// A Trace is a tree of spans.
type Trace struct {
	Span          // Root span
	Sub  []*Trace // Children
}

// String returns the Trace as a formatted string.
func (t *Trace) String() string {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// FindSpan recursively searches for a span whose Span ID is spanID in
// t and its descendants. If no such span is found, nil is returned.
func (t *Trace) FindSpan(spanID ID) *Trace {
	if t.ID.Span == spanID {
		return t
	}
	for _, sub := range t.Sub {
		if s := sub.FindSpan(spanID); s != nil {
			return s
		}
	}
	return nil
}

// TreeString returns the Trace as a formatted string that visually
// represents the trace's tree.
func (t *Trace) TreeString() string {
	var buf bytes.Buffer
	t.treeString(&buf, 0)
	return buf.String()
}

func (t *Trace) treeString(w io.Writer, depth int) {
	const indent1 = "    "
	indent := strings.Repeat(indent1, depth)

	if depth == 0 {
		fmt.Fprintf(w, "+ Trace %x\n", uint64(t.Span.ID.Trace))
	} else {
		if depth == 1 {
			fmt.Fprint(w, "|")
		} else {
			fmt.Fprint(w, "|", indent[len(indent1):])
		}
		fmt.Fprintf(w, "%s+ Span %x", strings.Repeat("-", len(indent1)), uint64(t.Span.ID.Span))
		if t.Span.ID.Parent != 0 {
			fmt.Fprintf(w, " (parent %x)", uint64(t.Span.ID.Parent))
		}
		fmt.Fprintln(w)
	}
	for _, a := range t.Span.Annotations {
		if depth == 0 {
			fmt.Fprint(w, "| ")
		} else {
			fmt.Fprint(w, "|", indent[1:], " | ")
		}
		fmt.Fprintf(w, "%s = %s\n", a.Key, a.Value)
	}
	for _, sub := range t.Sub {
		sub.treeString(w, depth+1)
	}
}

type tracesByIDSpan []*Trace

func (t tracesByIDSpan) Len() int           { return len(t) }
func (t tracesByIDSpan) Less(i, j int) bool { return t[i].Span.ID.Span < t[j].Span.ID.Span }
func (t tracesByIDSpan) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
