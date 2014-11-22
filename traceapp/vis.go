package traceapp

import (
	"time"

	"sourcegraph.com/sourcegraph/apptrace"

	_ "sourcegraph.com/sourcegraph/apptrace/httptrace"
)

type timelineItem struct {
	Label string                 `json:"label"`
	Times []timelineItemTimespan `json:"times"`
	Data  map[string]string      `json:"-"`
}

type timelineItemTimespan struct {
	Label string `json:"label"`
	Start int64  `json:"starting_time"` // msec since epoch
	End   int64  `json:"ending_time"`   // msec since epoch
}

func d3timeline(t *apptrace.Trace) ([]timelineItem, error) {
	var items []timelineItem

	var events []apptrace.Event
	if err := apptrace.UnmarshalEvents(t.Span.Annotations, &events); err != nil {
		return nil, err
	}

	item := timelineItem{Label: t.Span.Name(), Data: t.Annotations.StringMap()}
	for _, e := range events {
		if e, ok := e.(apptrace.TimespanEvent); ok {
			start := e.Start().UnixNano() / int64(time.Millisecond)
			end := e.End().UnixNano() / int64(time.Millisecond)
			ts := timelineItemTimespan{
				Label: t.Span.Name(),
				Start: start,
				End:   end,
			}
			if item.Times == nil {
				item.Times = append(item.Times, ts)
			} else {
				if item.Times[0].Start > start {
					item.Times[0].Start = start
				}
				if item.Times[0].End < end {
					item.Times[0].End = end
				}
			}
		}
	}
	items = append(items, item)

	for _, child := range t.Sub {
		subItems, err := d3timeline(child)
		if err != nil {
			return nil, err
		}
		items = append(items, subItems...)
	}

	return items, nil
}
