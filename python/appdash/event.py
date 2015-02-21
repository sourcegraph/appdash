import strict_rfc3339
import time
from spanid import *

__schemaPrefix = "_schema:"

# timeString returns a appdash-compatable (RFC 3339 / UTC offset) time string
# for the given timestamp (float seconds since epoch).
def timeString(ts):
    return strict_rfc3339.timestamp_to_rfc3339_utcoffset(ts)

# MarshalEvent marshals an event into annotations.
def MarshalEvent(e):
    a = []
    for key, value in e.marshal().items():
        a.append(Annotation(key, value))
    a.append(Annotation(__schemaPrefix + e.schema, ""))
    return a

# SpanNameEvent is an event which sets a span's name.
class SpanNameEvent:
    schema = "name"

    # name is literally the span's name.
    name = ""

    def __init__(self, name):
        self.name = name

    # marshal returns a dictionary of this event's value by name.
    def marshal(self):
        return {"Name": self.name}

# MsgEvent is an event that contains only a human-readable message.
class MsgEvent:
    schema = "msg"

    # msg is literally the message string.
    msg = ""

    def __init__(self, msg):
        self.msg = msg

    # marshal returns a dictionary of this event's value by schema.
    def marshal(self):
        return {"Msg": self.msg}

# LogEvent is an event whose timestamp is the current time and contains the
# given human-readable log message.
class LogEvent:
    schema = "log"

    # msg is literally the message string.
    msg = ""

    # RFC3339-UTC timestamp of the event.
    time = ""

    def __init__(self, msg):
        self.msg = msg
        self.time = timeString(time.time())

    def marshal(self):
        return {"Msg": self.msg, "Time": self.time}

# SQLEvent is an SQL query event with send and receive times, as well as the
# actual SQL that was ran, and a optional tag.
class SQLEvent:
    schema = "SQL"

    # sql is literally the SQL query that was ran.
    sql = ""

    # tag is a optional user-created tag associated with the SQL event. 
    tag = ""

    # RFC3339-UTC timestamp of when the query was sent, and later a result received.
    clientSend = ""
    clientRecv = ""

    def __init__(self, sql, send, recv=None, tag=""):
        self.sql = sql
        self.tag = tag
        self.clientSend = timeString(send)

        # If user didn't specify a recv time, use right now.
        if recv:
            self.clientRecv = timeString(recv)
        else:
            self.clientRecv = timeString(time.time())

    def marshal(self):
        return {
            "SQL": self.sql,
            "Tag": self.tag,
            "ClientSend": self.clientSend,
            "ClientRecv": self.clientRecv,
        }

# TODO(slimsag): add HTTPEvent
