package resp

import (
	"fmt"
	"io"
)

// Writer serializes Values into RESP wire format. It's deliberately
// symmetrical with Reader: every Type that can be parsed can also be
// written, so a server can echo any value type back as a response.
type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write serializes v and writes it out. Arrays recurse into Write
// for each element, mirroring how Reader.Read recurses.
func (w *Writer) Write(v Value) error {
	switch v.Type {
	case SimpleString:
		_, err := fmt.Fprintf(w.w, "+%s\r\n", v.Str)
		return err
	case Error:
		_, err := fmt.Fprintf(w.w, "-%s\r\n", v.Str)
		return err
	case Integer:
		_, err := fmt.Fprintf(w.w, ":%d\r\n", v.Int)
		return err
	case BulkString:
		if v.IsNull {
			_, err := w.w.Write([]byte("$-1\r\n"))
			return err
		}
		_, err := fmt.Fprintf(w.w, "$%d\r\n%s\r\n", len(v.Bulk), v.Bulk)
		return err
	case Array:
		if v.IsNull {
			_, err := w.w.Write([]byte("*-1\r\n"))
			return err
		}
		if _, err := fmt.Fprintf(w.w, "*%d\r\n", len(v.Elements)); err != nil {
			return err
		}
		for _, elem := range v.Elements {
			if err := w.Write(elem); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("resp: cannot write unknown type %q", v.Type)
	}
}
