# appdash

<img width=250 src="https://s3-us-west-2.amazonaws.com/sourcegraph-assets/apptrace-screenshot0.png" align="right">

Appdash is an application tracing system for Go, based on
[Google's Dapper](http://research.google.com/pubs/pub36356.html) and
[Twitter's Zipkin](http://twitter.github.io/zipkin/).

Appdash allows you to trace the end-to-end handling of requests and
operations in your application (for perf and debugging). It displays
timings and application-specific metadata for each step, and it
displays a tree and timeline for each request and its children.

To use appdash, you must instrument your application with calls to an
appdash recorder. You can record any type of event or
operation. Recorders and schemas for HTTP (client and server) and SQL
are provided, and you can write your own.


## Usage

To install appdash, run:

```
go get sourcegraph.com/sourcegraph/appdash/cmd/...
```

Check `cmd/appdash/example_app.go` for an example Web app that uses
appdash. Run `appdash demo` to run the app.


## Community

Questions or comments? Join us [on Slack](http://slackin.srclib.org/)!

## Development

Appdash uses [go-bindata](https://github.com/jteeuwen/go-bindata) to package HTML templates with the appdash binaryfor
distribution. To generate a binary that includes the template data, run `make gen-dist` to re-generate the `data.go`
file.

## Components

Appdash follows the design and naming conventions of
[Google's Dapper](http://research.google.com/pubs/pub36356.html). You
should read that paper if you are curious about why certain
architectural choices were made.

There are 4 main components/concepts in appdash:

*
  [**Spans**](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/.def/SpanID):
  A span refers to an operation and all of its children. For example,
  an HTTP handler handles a request by calling other components in
  your system, which in turn make various API and DB calls. The HTTP
  handler's span includes all downstream operations and their
  descendents; likewise, each downstream operation is its own span and
  has its own descendents. In this way, appdash constructs a tree of
  all of the operations that occur during the handling of the HTTP
  request.
* [**Event**](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/.def/Event):
  Your application records the various operations it performs (in the
  course of handling a request) as Events. Events can be arbitrary
  messages or metadata, or they can be structured event types defined
  by a Go type (such as an HTTP
  [ServerEvent](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/httptrace/.def/ServerEvent)
  or an
  [SQLEvent](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/sqltrace/.def/SQLEvent)).
* [**Recorder**](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/.def/Recorder):
  Your application uses a Recorder to send events to a Collector (see
  below). Each Recorder is associated with a particular span in the
  tree of operations that are handling a particular request, and all
  events sent via a Recorder are automatically associated with that
  context.
* [**Collector**](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/.def/Collector):
  A Collector receives Annotations (which are the encoded form of
  Events) sent by a Recorder. Typically, your application's Recorder
  talks to a local Collector (created with
  [NewRemoteCollector](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/.def/NewRemoteCollector). This
  local Collector forwards data to a remote appdash server (created
  with
  [NewServer](https://sourcegraph.com/sourcegraph.com/sourcegraph/appdash@master/.GoPackage/sourcegraph.com/sourcegraph/appdash/.def/NewServer)
  that combines traces from all of the services that compose your
  application. The appdash server in turn runs a Collector that
  listens on the network for this data, and it then stores what it
  receives.



## Acknowledgments

**appdash** was influenced by, and uses code from, Coda Hale's
[lunk](https://github.com/codahale/lunk).
