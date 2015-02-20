#!/usr/bin/python

# Twisted imports
from twisted.internet import reactor

# Appdash imports
import appdash

# Create a remote appdash collector.
collector = appdash.RemoteCollectorFactory(reactor, debug=True)

# Create a trace.
trace = appdash.SpanID(root=True)

# Generate a few spans with some annotations.
span = trace
for i in range(0, 7):
    # Collect some annotations.
    a1 = appdash.Annotation(
        key="HTTPClient",
        value="/foo/bar?eq=1",
    )
    a2 = appdash.Annotation(
        key="SQL",
        value="select * from table;",
    )
    collector.collect(span, a1, a2)

    # Create a new child span whose parent is the last span we created.
    span = appdash.SpanID(parent=span)

# Have Twisted perform the connection and run.
reactor.connectTCP("", 7701, collector)
reactor.run()
