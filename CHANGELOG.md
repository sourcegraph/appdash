## v0.1.0 [unreleased]

### Release Notes

There are breaking changes in this release:
- `AggregateStore` is deprecated in favor of `InfluxDBStore`:
  - Please refer to the [demo example](https://github.com/sourcegraph/appdash/blob/master/examples/cmd/webapp-influxdb/main.go#L50) for further information on how to migrate to `InfluxDBStore`.

### Features

- [#99](https://github.com/sourcegraph/appdash/pull/99): New store engine backed by [InfluxDB](https://github.com/influxdata/influxdb).
- [#110](https://github.com/sourcegraph/appdash/pull/110): Implementation of the OpenTracing API.
