package resp

import (
	"strings"
	"testing"
)

// Table-driven test: each entry is one scenario — a name (shows up
// in test output if it fails, so make it descriptive), an input
// string as it would appear on the wire, and what we expect Read()
// to produce from it.
func TestReader_SimpleString(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "OK reply", input: "+OK\r\n", want: "OK"},
		{name: "PONG reply", input: "+PONG\r\n", want: "PONG"},
		{name: "empty simple string", input: "+\r\n", want: ""},
	}

	for _, tc := range cases {
		// t.Run gives each case its own named sub-test. Two big
		// wins: if "empty simple string" fails, `go test` tells you
		// exactly that one failed (not just "TestReader_SimpleString
		// failed" with no idea which case) — and you can re-run just
		// that one case with `go test -run TestReader_SimpleString/empty_simple_string`.
		t.Run(tc.name, func(t *testing.T) {
			// strings.NewReader turns our test string into an
			// io.Reader — the same interface a real net.Conn
			// satisfies, so from Reader's point of view this is
			// indistinguishable from bytes arriving over a real
			// socket. That's what makes this a *unit* test: we're
			// exercising the real parsing logic, we've just swapped
			// out "a TCP connection" for "a string in memory" as the
			// source of bytes.
			r := NewReader(strings.NewReader(tc.input))

			got, err := r.Read()
			if err != nil {
				t.Fatalf("Read() returned unexpected error: %v", err)
			}

			if got.Type != SimpleString {
				t.Errorf("Type = %v, want SimpleString", got.Type)
			}
			if got.Str != tc.want {
				t.Errorf("Str = %q, want %q", got.Str, tc.want)
			}
		})
	}
}

func TestReader_Integer(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int64
	}{
		{name: "positive integer", input: ":1000\r\n", want: 1000},
		{name: "zero", input: ":0\r\n", want: 0},
		{name: "negative integer", input: ":-5\r\n", want: -5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tc.input))

			got, err := r.Read()
			if err != nil {
				t.Fatalf("Read() returned unexpected error: %v", err)
			}

			if got.Type != Integer {
				t.Errorf("Type = %v, want Integer", got.Type)
			}
			if got.Int != tc.want {
				t.Errorf("Int = %d, want %d", got.Int, tc.want)
			}
		})
	}
}

// Not every test should expect success — testing that bad input is
// correctly REJECTED is just as important as testing good input
// works. This is worth calling out explicitly: it's a common blind
// spot to only ever write tests for the happy path.
func TestReader_Integer_InvalidInput(t *testing.T) {
	r := NewReader(strings.NewReader(":not-a-number\r\n"))

	_, err := r.Read()
	if err == nil {
		t.Error("Read() with invalid integer text: expected an error, got nil")
	}
}

func TestReader_BulkString(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   string
		IsNull bool
	}{
		{name: "Normal Bulk String", input: "$5\r\nhello\r\n", want: "hello", IsNull: false},
		{name: "Null Bulk String", input: "$-1\r\n", want: "", IsNull: true},
		{name: "Empty But Not Null Bulk String", input: "$0\r\n\r\n", want: "", IsNull: false},
		}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tc.input))

			got, err := r.Read()
			if err != nil {
				t.Fatalf("Read() returned unexpected error: %v", err)
			}

			if got.Type != BulkString{
				t.Errorf("Type = %v, want BulkString", got.Type)
			}
			if got.IsNull != tc.IsNull{
				t.Errorf("Bulk IsNull = %v, want %v", got.IsNull, tc.IsNull)
			}
			if got.Bulk != tc.want {
				t.Errorf("Bulk = %q, want %q", got.Bulk, tc.want)
			}
		})
	}
}

func TestReader_BulkString_InvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "Invalid Bulk String", input: "$-5\r\n"},
		{name: "maxBulkLen errors", input: "$536870913\r\n"},
		}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tc.input))

			_, err := r.Read()
			if err == nil {
				t.Error("Read() with invalid BulkString: expected an error, got nil")
			}
		})
	}
}
