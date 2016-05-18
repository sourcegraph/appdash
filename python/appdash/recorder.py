from spanid import SpanID, Annotation

class AppdashRecorder(object):
    def __init__(self, collector):
        self._collector = collector

    def record_span(self, span):
        if not span.context.sampled:
            return
        annotations = []
        span_id = SpanID()
        span_id.trace = span.context.trace_id
        span_id.span = span.context.span
        span_id.parent = span.context.parent_span_id

        # It might not be right to build a list, maybe send events one at a time.
        annotations.append(MarshalEvent(SpanNameEvent(span.operation_name)))

        approx_endtime = span.context.start_time + span.duration
        annotations.append(MarshalEvent(
            TimespanEvent(span.context.start_time, approx_endtime)))

        for key in span.tags:
            annotations.append(Annotation(key, span.tag[key])

        for key in span.context.baggage_items:
            annotations.append(Annotation(key, span.contex.baggage_items[value])

        self._collector.collect(span_id, events)


class AppdashTracer(BasicTracer):

    def __init__(self, collector, sampler=None):
        self._recorder = AppdashRecorder(collector)
        super(AppdashTracer, self).__init__(recorder=self._recorder,
                                            sampler=sampler)
