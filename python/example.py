#!/usr/bin/python

# Twisted imports
from twisted.internet import reactor

# Appdash imports
import appdash

# Create a remote appdash collector.
collector = appdash.RemoteCollectorFactory(reactor, debug=True)


# TODO(slimsag): do not require direct usage of protobuf.
import appdash.collector_pb2 as wire

# Send a few packets.
traceID = appdash.generateID()
for i in range(0, 7):
    # Create a collect packet
    p = wire.CollectPacket()
    p.spanid.trace = traceID
    p.spanid.span = appdash.generateID()
    p.spanid.parent = 0

    # Annotation 1
    a = p.annotation.add()
    a.key = "HTTPClient"
    a.value = "/foo/bar?eq=1"

    # Annotation 2
    a2 = p.annotation.add()
    a2.key = "SQL"
    a2.value = "select * from table;"

    # Collect the packet.
    collector.collect(p)


# Have Twisted perform the connection and run.
reactor.connectTCP("", 7701, collector)
reactor.run()
