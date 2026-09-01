package resp

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriter_SimpleString(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{name: "OK reply", value: SimpleStringValue("OK"), want: "+OK\r\n"},
		{name: "PONG reply", value: SimpleStringValue("PONG"), want: "+PONG\r\n"},
		{name: "empty simple string", value: SimpleStringValue(""), want: "+\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewWriter(&buf).Write(tc.value); err != nil {
				t.Fatalf("Write() returned unexpected error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("written RESP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriter_Error(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{name: "error message", value: ErrorValue("ERR unknown command"), want: "-ERR unknown command\r\n"},
		{name: "empty error message", value: ErrorValue(""), want: "-\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewWriter(&buf).Write(tc.value); err != nil {
				t.Fatalf("Write() returned unexpected error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("written RESP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriter_Integer(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{name: "positive integer", value: IntegerValue(1000), want: ":1000\r\n"},
		{name: "zero", value: IntegerValue(0), want: ":0\r\n"},
		{name: "negative integer", value: IntegerValue(-5), want: ":-5\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewWriter(&buf).Write(tc.value); err != nil {
				t.Fatalf("Write() returned unexpected error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("written RESP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriter_BulkString(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{name: "normal bulk string", value: BulkStringValue("hello"), want: "$5\r\nhello\r\n"},
		{name: "null bulk string", value: NullBulkString(), want: "$-1\r\n"},
		{name: "empty but not null bulk string", value: BulkStringValue(""), want: "$0\r\n\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewWriter(&buf).Write(tc.value); err != nil {
				t.Fatalf("Write() returned unexpected error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("written RESP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriter_Array(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{name: "normal command array", value: ArrayValue([]Value{
			BulkStringValue("SET"),
			BulkStringValue("foo"),
			BulkStringValue("bar"),
		}), want: "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"},
		{name: "array with null bulk string", value: ArrayValue([]Value{
			BulkStringValue("GET"),
			NullBulkString(),
		}), want: "*2\r\n$3\r\nGET\r\n$-1\r\n"},
		{name: "nested array", value: ArrayValue([]Value{
			IntegerValue(1),
			ArrayValue([]Value{SimpleStringValue("OK")}),
		}), want: "*2\r\n:1\r\n*1\r\n+OK\r\n"},
		{name: "null array", value: Value{Type: Array, IsNull: true}, want: "*-1\r\n"},
		{name: "empty array", value: ArrayValue([]Value{}), want: "*0\r\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := NewWriter(&buf).Write(tc.value); err != nil {
				t.Fatalf("Write() returned unexpected error: %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("written RESP = %q, want %q", got, tc.want)
			}
		})
	}
}

// An unknown Value.Type cannot be represented in RESP and must be rejected
// rather than writing a partial or ambiguous value.
func TestWriter_UnknownType(t *testing.T) {
	var buf bytes.Buffer
	err := NewWriter(&buf).Write(Value{Type: '?'})
	if err == nil {
		t.Fatal("Write() with an unknown type: expected an error, got nil")
	}
	if got := buf.String(); got != "" {
		t.Errorf("written RESP = %q, want no output", got)
	}
}

// failingWriter lets the test verify that Write returns an I/O error from its
// destination instead of hiding it from the server connection handler.
type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestWriter_PropagatesWriteError(t *testing.T) {
	wantErr := errors.New("connection closed")
	err := NewWriter(failingWriter{err: wantErr}).Write(SimpleStringValue("OK"))
	if !errors.Is(err, wantErr) {
		t.Errorf("Write() error = %v, want %v", err, wantErr)
	}
}
