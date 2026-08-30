package resp

// Type identifies which of the 5 RESP2 types a Value holds.
type Type byte

const (
	SimpleString Type = '+'
	Error        Type = '-'
	Integer      Type = ':'
	BulkString   Type = '$'
	Array        Type = '*'
)

// Value represents any RESP2 value. Only the fields relevant to its
// Type are meaningful — e.g. an Array's payload lives in Elements,
// a BulkString's in Bulk, and so on. This "one struct, tagged by
// type" approach is simpler to pass through the codebase than five
// separate types + an interface, at the cost of a few unused fields
// per instance, which is a fine trade at this scale.
type Value struct {
	Type     Type
	Str      string  // SimpleString, Error
	Int      int64   // Integer
	Bulk     string  // BulkString payload
	IsNull   bool    // true for a null bulk string ($-1\r\n) or null array (*-1\r\n)
	Elements []Value // Array
}

// Helper constructors — used constantly when building responses,
// so it's worth not hand-writing struct literals everywhere.

func SimpleStringValue(s string) Value { return Value{Type: SimpleString, Str: s} }
func ErrorValue(s string) Value        { return Value{Type: Error, Str: s} }
func IntegerValue(i int64) Value       { return Value{Type: Integer, Int: i} }
func BulkStringValue(s string) Value   { return Value{Type: BulkString, Bulk: s} }
func NullBulkString() Value            { return Value{Type: BulkString, IsNull: true} }
func ArrayValue(elems []Value) Value   { return Value{Type: Array, Elements: elems} }
