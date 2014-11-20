package apptrace

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMsg(t *testing.T) {
	e := Msg("foo")

	j, err := json.Marshal(e)
	if err != nil {
		t.Error(err)
	}

	got := string(j)
	want := `{"Msg":"foo"}`
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestMsg_Schema(t *testing.T) {
	e := Msg("foo")

	if e.Schema() != "msg" {
		t.Errorf("Unwant schema: %v", e.Schema())
	}
}

func TestLog(t *testing.T) {
	e := Log("foo").(logEvent)

	e.Time = time.Unix(123456789, 0).In(time.UTC)

	j, err := json.Marshal(e)
	if err != nil {
		t.Error(err)
	}

	got := string(j)
	want := `{"Msg":"foo","Time":"1973-11-29T21:33:09Z"}`
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestLog_Schema(t *testing.T) {
	e := Log("foo")

	if e.Schema() != "log" {
		t.Errorf("Unwant schema: %v", e.Schema())
	}
}
