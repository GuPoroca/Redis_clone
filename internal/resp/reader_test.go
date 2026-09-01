package resp

import (
	"reflect"
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
	cases := []struct {
		name  string
		input string
	}{
		{name: "Not a Number", input: ":not-a-number\r\n"},
		{name: "Integer has no terminator", input: ":42"},
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

// Errors are values in RESP (not Go errors). The leading '-' selects the
// Error type, while the message itself is stored in Value.Str.
func TestReader_Error(t *testing.T) {
	r := NewReader(strings.NewReader("-ERR unknown command\r\n"))

	got, err := r.Read()
	if err != nil {
		t.Fatalf("Read() returned unexpected error: %v", err)
	}
	if got.Type != Error {
		t.Errorf("Type = %v, want Error", got.Type)
	}
	if got.Str != "ERR unknown command" {
		t.Errorf("Str = %q, want %q", got.Str, "ERR unknown command")
	}
}

// A RESP value must start with one of the five type bytes. Rejecting an
// unrecognised byte prevents arbitrary stream data being treated as a value.
func TestReader_UnknownType(t *testing.T) {
	r := NewReader(strings.NewReader("?not-resp\r\n"))

	if _, err := r.Read(); err == nil {
		t.Error("Read() with an unknown type byte: expected an error, got nil")
	}
}

func TestReader_BulkString(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   string
		isNull bool
	}{
		{name: "Normal Bulk String", input: "$5\r\nhello\r\n", want: "hello", isNull: false},
		{name: "Null Bulk String", input: "$-1\r\n", want: "", isNull: true},
		{name: "Empty But Not Null Bulk String", input: "$0\r\n\r\n", want: "", isNull: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tc.input))

			got, err := r.Read()
			if err != nil {
				t.Fatalf("Read() returned unexpected error: %v", err)
			}

			if got.Type != BulkString {
				t.Errorf("Type = %v, want BulkString", got.Type)
			}
			if got.IsNull != tc.isNull {
				t.Errorf("Bulk IsNull = %v, want %v", got.IsNull, tc.isNull)
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
		// A bulk string always ends in CRLF. These two bytes must not be
		// accepted merely because the declared payload length was satisfied.
		{name: "Invalid bulk string terminator", input: "$3\r\nfooXX"},
		{name: "Invalid bulk string size", input: "$not-a-number\r\n"},
		{name: "bulk string truncated size", input: "$3\r\nhi"},
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

func TestReader_Array(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   []Value
		isNull bool
	}{
		{name: "Valid Normal Array", input: "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n", want: []Value{
			{Type: BulkString, Bulk: "SET"},
			{Type: BulkString, Bulk: "foo"},
			{Type: BulkString, Bulk: "bar"},
		}, isNull: false},
		{name: "Array with Null BulkString", input: "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$-1\r\n", want: []Value{
			{Type: BulkString, Bulk: "SET"},
			{Type: BulkString, Bulk: "foo"},
			{Type: BulkString, Bulk: "", IsNull: true},
		}, isNull: false},
		{name: "Array with Nested Array", input: "*2\r\n:1\r\n*1\r\n+OK\r\n", want: []Value{
			{Type: Integer, Int: 1},
			{Type: Array, Elements: []Value{
				{Type: SimpleString, Str: "OK"},
			}},
		}, isNull: false},
		{name: "Null Array", input: "*-1\r\n", want: nil, isNull: true},
		{name: "Empty Array", input: "*0\r\n", want: []Value{}, isNull: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tc.input))

			got, err := r.Read()
			if err != nil {
				t.Fatalf("Read() returned unexpected error: %v", err)
			}

			if got.Type != Array {
				t.Errorf("Type = %v, want Array", got.Type)
			}
			if got.IsNull != tc.isNull {
				t.Errorf("Array IsNull = %v, want %v", got.IsNull, tc.isNull)
			}
			if !reflect.DeepEqual(got.Elements, tc.want) {
				t.Errorf("Elements = %#v, want %#v", got.Elements, tc.want)
			}
		})
	}
}

func TestReader_Array_InvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "Invalid Array", input: "*-5\r\n"},
		{name: "Non-numeric array length", input: "*not-a-number\r\n"},
		{name: "maxArrayLen errors", input: "*1048577\r\n"},
		{name: "truncated array errors", input: "*"},
		{name: "array with less elements than it says", input: "*4\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReader(strings.NewReader(tc.input))

			_, err := r.Read()
			if err == nil {
				t.Error("Read() with invalid Array: expected an error, got nil")
			}
		})
	}
}

// RESP is a stream protocol: two values can arrive back-to-back on the same
// connection. Each Read must consume exactly one value and leave the next one
// available for the following call.
func TestReader_ConsecutiveValues(t *testing.T) {
	r := NewReader(strings.NewReader("+OK\r\n:42\r\n"))

	first, err := r.Read()
	if err != nil {
		t.Fatalf("first Read() returned unexpected error: %v", err)
	}
	if first.Type != SimpleString || first.Str != "OK" {
		t.Errorf("first value = %#v, want simple string OK", first)
	}

	second, err := r.Read()
	if err != nil {
		t.Fatalf("second Read() returned unexpected error: %v", err)
	}
	if second.Type != Integer || second.Int != 42 {
		t.Errorf("second value = %#v, want integer 42", second)
	}
}
