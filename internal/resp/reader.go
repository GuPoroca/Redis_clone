package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Reader parses RESP values from a stream. It wraps a bufio.Reader
// so it can read byte-by-byte and line-by-line cheaply.
type Reader struct {
	r *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

// Read parses one complete RESP value, recursing for Arrays.
func (r *Reader) Read() (Value, error) {
	typeByte, err := r.r.ReadByte()
	if err != nil {
		return Value{}, err // typically io.EOF — caller treats as disconnect
	}

	switch Type(typeByte) {
	case SimpleString:
		s, err := r.readLine()
		return Value{Type: SimpleString, Str: s}, err
	case Error:
		s, err := r.readLine()
		return Value{Type: Error, Str: s}, err
	case Integer:
		s, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("resp: invalid integer %q: %w", s, err)
		}
		return Value{Type: Integer, Int: n}, nil
	case BulkString:
		return r.readBulkString()
	case Array:
		return r.readArray()
	default:
		return Value{}, fmt.Errorf("resp: unknown type byte %q", typeByte)
	}
}

// readLine reads until \r\n and returns the line without the
// terminator. RESP uses \r\n as the delimiter for every type, not
// just plain \n, so this can't just be bufio.Scanner's default.
func (r *Reader) readLine() (string, error) {
	line, err := r.r.ReadString('\n')
	if err != nil {
		return "", err
	}
	// Trim the trailing \r\n. Being defensive about a bare \n here
	// (no \r) is deliberate — some hand-typed test input over `nc`
	// won't send \r, and failing loudly on that during development
	// would be more annoying than helpful.
	n := len(line)
	if n >= 2 && line[n-2] == '\r' {
		return line[:n-2], nil
	}
	return line[:n-1], nil
}

func (r *Reader) readBulkString() (Value, error) {
	lengthLine, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	length, err := strconv.Atoi(lengthLine)
	if err != nil {
		return Value{}, fmt.Errorf("resp: invalid bulk string length %q: %w", lengthLine, err)
	}
	if length == -1 {
		// $-1\r\n is RESP's "null" bulk string — used for things like
		// GET on a missing key. No trailing data or \r\n follows it.
		return NullBulkString(), nil
	}

	buf := make([]byte, length+2) // +2 for the trailing \r\n
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return Value{}, err
	}
	return Value{Type: BulkString, Bulk: string(buf[:length])}, nil
}

func (r *Reader) readArray() (Value, error) {
	countLine, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	count, err := strconv.Atoi(countLine)
	if err != nil {
		return Value{}, fmt.Errorf("resp: invalid array length %q: %w", countLine, err)
	}
	if count == -1 {
		return Value{Type: Array, IsNull: true}, nil
	}

	elements := make([]Value, count)
	for i := 0; i < count; i++ {
		v, err := r.Read()
		if err != nil {
			return Value{}, err
		}
		elements[i] = v
	}
	return Value{Type: Array, Elements: elements}, nil
}
