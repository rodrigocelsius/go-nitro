package runtime

import "strconv"

// AppendStringField writes a string field as "key":"value" using
// append directly (no fmt.Sprintf).
func AppendStringField(buf []byte, key, value string) []byte {
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':')
	buf = append(buf, '"')
	buf = AppendJSONEscapedString(buf, value)
	buf = append(buf, '"')
	return buf
}

// AppendIntField writes an int field as "key":value.
func AppendIntField(buf []byte, key string, value int) []byte {
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':')
	buf = strconv.AppendInt(buf, int64(value), 10)
	return buf
}

// AppendInt64Field writes an int64 field as "key":value.
func AppendInt64Field(buf []byte, key string, value int64) []byte {
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':')
	buf = strconv.AppendInt(buf, value, 10)
	return buf
}

// AppendFloat64Field writes a float64 field as "key":value.
func AppendFloat64Field(buf []byte, key string, value float64) []byte {
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':')
	buf = strconv.AppendFloat(buf, value, 'f', -1, 64)
	return buf
}

// AppendBoolField writes a bool field as "key":true/false.
func AppendBoolField(buf []byte, key string, value bool) []byte {
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':')
	buf = strconv.AppendBool(buf, value)
	return buf
}

// AppendNullField writes a null field as "key":null.
func AppendNullField(buf []byte, key string) []byte {
	buf = append(buf, '"')
	buf = append(buf, key...)
	buf = append(buf, '"', ':', 'n', 'u', 'l', 'l')
	return buf
}

// AppendObjectStart writes '{'.
func AppendObjectStart(buf []byte) []byte {
	return append(buf, '{')
}

// AppendObjectEnd writes '}'.
func AppendObjectEnd(buf []byte) []byte {
	return append(buf, '}')
}

// AppendComma writes ','.
func AppendComma(buf []byte) []byte {
	return append(buf, ',')
}
