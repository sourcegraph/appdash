package appdash

import influxdb "github.com/influxdb/influxdb/cmd/influxd/run"

// Compile-time "implements" check.
var _ interface {
	Store
	Queryer
} = (*InfluxDBStore)(nil)

type InfluxDBStore struct {
}

func (in *InfluxDBStore) Collect(id SpanID, anns ...Annotation) error {
	return nil
}

func (in *InfluxDBStore) Trace(id ID) (*Trace, error) {
	return nil, nil
}

func (in *InfluxDBStore) Traces() ([]*Trace, error) {
	return nil, nil
}

func NewInfluxDBStore(c *influxdb.Config, buildInfo *influxdb.BuildInfo) (*InfluxDBStore, error) {
	s, err := influxdb.NewServer(c, buildInfo)
	if err != nil {
		return nil, err
	}
	if err := s.Open(); err != nil {
		return nil, err
	}
	return &InfluxDBStore{}, nil
}
