# apptrace

<img width=250 src="https://s3-us-west-2.amazonaws.com/sourcegraph-assets/apptrace-screenshot0.png" align="right">

Apptrace is an application tracing system for Go, based on
[Google's Dapper](http://research.google.com/pubs/pub36356.html) and
[Twitter's Zipkin](http://twitter.github.io/zipkin/).

Apptrace allows you to trace the end-to-end handling of requests and
operations in your application (for perf and debugging). It displays
timings and application-specific metadata for each step, and it
displays a tree and timeline for each request and its children.

To use apptrace, you must instrument your application with calls to an
apptrace recorder. You can record any type of event or
operation. Recorders and schemas for HTTP (client and server) and SQL
are provided, and you can write your own.


## Usage

To install apptrace, run:

```
go get sourcegraph.com/sourcegraph/apptrace/...
```

Check `cmd/apptrace/example_app.go` for an example Web app that uses
apptrace. Run `apptrace demo` to run the app.


## Development

apptrace uses [go-bindata](https://github.com/jteeuwen/go-bindata) to package HTML templates with the apptrace binary.
When developing, run `make gen-dev`. This will modify the `data.go` to read directly from the template source files,
which makes it easier to test/debug as changes to templates will be reflected immediately in the running app. Do not
push these changes upstream.

When pushing changes to template files, run `make gen-dist` to re-generate the `data.go` file and push these changes
together with the changes to the template files.

## Components

Apptrace follows the design and naming conventions of
[Google's Dapper](http://research.google.com/pubs/pub36356.html). You
should read that paper if you are curious about why certain
architectural choices were made.

There are 4 main components/concepts in apptrace:

*
  [**Spans**](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/.def/SpanID):
  A span refers to an operation and all of its children. For example,
  an HTTP handler handles a request by calling other components in
  your system, which in turn make various API and DB calls. The HTTP
  handler's span includes all downstream operations and their
  descendents; likewise, each downstream operation is its own span and
  has its own descendents. In this way, apptrace constructs a tree of
  all of the operations that occur during the handling of the HTTP
  request.
* [**Event**](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/.def/Event):
  Your application records the various operations it performs (in the
  course of handling a request) as Events. Events can be arbitrary
  messages or metadata, or they can be structured event types defined
  by a Go type (such as an HTTP
  [ServerEvent](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/httptrace/.def/ServerEvent)
  or an
  [SQLEvent](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/sqltrace/.def/SQLEvent)).
* [**Recorder**](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/.def/Recorder):
  Your application uses a Recorder to send events to a Collector (see
  below). Each Recorder is associated with a particular span in the
  tree of operations that are handling a particular request, and all
  events sent via a Recorder are automatically associated with that
  context.
* [**Collector**](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/.def/Collector):
  A Collector receives Annotations (which are the encoded form of
  Events) sent by a Recorder. Typically, your application's Recorder
  talks to a local Collector (created with
  [NewRemoteCollector](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/.def/NewRemoteCollector). This
  local Collector forwards data to a remote apptrace server (created
  with
  [NewServer](https://sourcegraph.com/sourcegraph.com/sourcegraph/apptrace@master/.GoPackage/sourcegraph.com/sourcegraph/apptrace/.def/NewServer)
  that combines traces from all of the services that compose your
  application. The apptrace server in turn runs a Collector that
  listens on the network for this data, and it then stores what it
  receives.



## Acknowledgments

**apptrace** was influenced by, and uses code from, Coda Hale's
[lunk](https://github.com/codahale/lunk).
