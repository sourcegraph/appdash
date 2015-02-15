from twisted.internet.protocol import Protocol, ReconnectingClientFactory
import collector_pb2 as wire
import varint
from sys import stdout

class CollectorProtocol(Protocol):
    # CollectorProtocol is a Twisted implementation of appdash's protobuf-based
    # collector protocol.

    # writeMsg writes the protobuf message out to the protocol's transport. It
    # is preceded with the varint-encoded length of the data (i.e. a delimited
    # protobuf message).
    def writeMsg(self, msg):
        data = msg.SerializeToString()
        self.transport.write(varint.encode(len(data)))
        self.transport.write(data)

    def connectionMade(self):
        self._factory._log('connected!')

    def connectionLost(self, reason):
        self._factory._log("disconnected.", reason.getErrorMessage())

    def dataReceived(self, data):
        self._factory._log('got', len(data), 'bytes of unexpected data from server.')


class RemoteCollectorFactory(ReconnectingClientFactory):
    # RemoteCollectorFactory is a Twisted factory for remote collectors, which
    # collect spans and their annotations, sending them to a remote Go appdash
    # server for collection. After collection they can be viewed in appdash's
    # web user interface.

    _reactor = None
    _debug = False
    _remote = None
    _pending = []

    def __init__(self, reactor, debug=False):
        self._reactor = reactor
        self._debug = debug

    def _log(self, *args):
        if self._debug:
            print "appdash: %s" % (" ".join(args))

    # collect collects a packet to be flushed at a later time, when connection
    # has been made.
    #
    # TODO(slimsag): We shouldn't blindly take a protobuf CollectionPacket. It
    # is too low-level for clients.
    def collect(self, packet):
        self._pending.append(packet)
        if self._remote != None:
            self._reactor.callLater(1/2, self._flush)

    # _flush is called internally after either a new collection has occured, or
    # after connection has been made with the remote server. It writes all the
    # pending messages out to the remote.
    def _flush(self):
        self._log("flushing", str(len(self._pending)), "messages")
        for p in self._pending:
            self._remote.writeMsg(p)
        self._pending = []

    def startedConnecting(self, connector):
        self._log('connecting..')

    def buildProtocol(self, addr):
        # Reset delay to reconnection -- otherwise it's exponential (which is
        # not a good match for us).
        self.resetDelay()

        # Create the protocol.
        p = CollectorProtocol()
        p._factory = self
        self._remote = p
        self._reactor.callLater(1/2, self._flush)
        return p

    def clientConnectionFailed(self, connector, reason):
        self._log('connection failed:', reason.getErrorMessage())
        ReconnectingClientFactory.clientConnectionFailed(self, connector, reason)

