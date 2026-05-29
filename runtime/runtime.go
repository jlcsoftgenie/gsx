package runtime

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

type HTML string

var bufferPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func WriteString(w io.Writer, s string) error {
	_, err := writeString(w, s)
	return err
}

func WriteEscapedString(w io.Writer, s string) error {
	return writeEscapedString(w, s)
}

func WriteEscapedBytes(w io.Writer, b []byte) error {
	return writeEscapedBytes(w, b)
}

func WriteBool(w io.Writer, v bool) error {
	if v {
		return WriteString(w, "true")
	}
	return WriteString(w, "false")
}

func WriteInt64(w io.Writer, v int64) error {
	var buf [20]byte
	out := strconv.AppendInt(buf[:0], v, 10)
	_, err := w.Write(out)
	return err
}

func WriteUint64(w io.Writer, v uint64) error {
	var buf [20]byte
	out := strconv.AppendUint(buf[:0], v, 10)
	_, err := w.Write(out)
	return err
}

func WriteFloat64(w io.Writer, v float64) error {
	var buf [32]byte
	out := strconv.AppendFloat(buf[:0], v, 'f', -1, 64)
	_, err := w.Write(out)
	return err
}

func WriteEscaped(w io.Writer, v any) error {
	switch x := v.(type) {
	case string:
		return WriteEscapedString(w, x)
	case []byte:
		return WriteEscapedBytes(w, x)
	case HTML:
		return WriteEscapedString(w, string(x))
	case fmt.Stringer:
		return WriteEscapedString(w, x.String())
	case int:
		return WriteInt64(w, int64(x))
	case int8:
		return WriteInt64(w, int64(x))
	case int16:
		return WriteInt64(w, int64(x))
	case int32:
		return WriteInt64(w, int64(x))
	case int64:
		return WriteInt64(w, x)
	case uint:
		return WriteUint64(w, uint64(x))
	case uint8:
		return WriteUint64(w, uint64(x))
	case uint16:
		return WriteUint64(w, uint64(x))
	case uint32:
		return WriteUint64(w, uint64(x))
	case uint64:
		return WriteUint64(w, x)
	case float32:
		return WriteFloat64(w, float64(x))
	case float64:
		return WriteFloat64(w, x)
	case bool:
		return WriteBool(w, x)
	}
	return nil
}

func WriteAttrString(w io.Writer, name string, v string) error {
	if v == "" {
		return nil
	}
	if err := writeAttrPrefix(w, name); err != nil {
		return err
	}
	if err := WriteEscapedString(w, v); err != nil {
		return err
	}
	return writeAttrSuffix(w)
}

func WriteAttrBytes(w io.Writer, name string, v []byte) error {
	if len(v) == 0 {
		return nil
	}
	if err := writeAttrPrefix(w, name); err != nil {
		return err
	}
	if err := WriteEscapedBytes(w, v); err != nil {
		return err
	}
	return writeAttrSuffix(w)
}

func WriteAttrBool(w io.Writer, name string, v bool) error {
	if err := writeAttrPrefix(w, name); err != nil {
		return err
	}
	if err := WriteBool(w, v); err != nil {
		return err
	}
	return writeAttrSuffix(w)
}

func WriteBoolAttr(w io.Writer, name string, v bool) error {
	if !v {
		return nil
	}
	if err := WriteString(w, " "); err != nil {
		return err
	}
	return WriteString(w, name)
}

func WriteAttrInt64(w io.Writer, name string, v int64) error {
	if err := writeAttrPrefix(w, name); err != nil {
		return err
	}
	if err := WriteInt64(w, v); err != nil {
		return err
	}
	return writeAttrSuffix(w)
}

func WriteAttrUint64(w io.Writer, name string, v uint64) error {
	if err := writeAttrPrefix(w, name); err != nil {
		return err
	}
	if err := WriteUint64(w, v); err != nil {
		return err
	}
	return writeAttrSuffix(w)
}

func WriteAttrFloat64(w io.Writer, name string, v float64) error {
	if err := writeAttrPrefix(w, name); err != nil {
		return err
	}
	if err := WriteFloat64(w, v); err != nil {
		return err
	}
	return writeAttrSuffix(w)
}

func WriteAttr(w io.Writer, name string, v any, boolean bool) error {
	if v == nil {
		return nil
	}
	if boolean {
		if b, ok := v.(bool); ok {
			return WriteBoolAttr(w, name, b)
		}
	}
	switch x := v.(type) {
	case string:
		return WriteAttrString(w, name, x)
	case []byte:
		return WriteAttrBytes(w, name, x)
	case HTML:
		return WriteAttrString(w, name, string(x))
	case fmt.Stringer:
		return WriteAttrString(w, name, x.String())
	case int:
		return WriteAttrInt64(w, name, int64(x))
	case int8:
		return WriteAttrInt64(w, name, int64(x))
	case int16:
		return WriteAttrInt64(w, name, int64(x))
	case int32:
		return WriteAttrInt64(w, name, int64(x))
	case int64:
		return WriteAttrInt64(w, name, x)
	case uint:
		return WriteAttrUint64(w, name, uint64(x))
	case uint8:
		return WriteAttrUint64(w, name, uint64(x))
	case uint16:
		return WriteAttrUint64(w, name, uint64(x))
	case uint32:
		return WriteAttrUint64(w, name, uint64(x))
	case uint64:
		return WriteAttrUint64(w, name, x)
	case float32:
		return WriteAttrFloat64(w, name, float64(x))
	case float64:
		return WriteAttrFloat64(w, name, x)
	case bool:
		return WriteAttrBool(w, name, x)
	}
	return nil
}

func WriteRaw(w io.Writer, v HTML) error {
	return WriteString(w, string(v))
}

func RenderString(fn func(io.Writer) error) (string, error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)
	if err := fn(buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeEscapedString(w io.Writer, s string) error {
	first := strings.IndexAny(s, "&<>\"'")
	if first < 0 {
		_, err := writeString(w, s)
		return err
	}
	last := 0
	for i := first; i < len(s); i++ {
		repl := ""
		switch s[i] {
		case '&':
			repl = "&amp;"
		case '<':
			repl = "&lt;"
		case '>':
			repl = "&gt;"
		case '"':
			repl = "&#34;"
		case '\'':
			repl = "&#39;"
		}
		if repl == "" {
			continue
		}
		if last < i {
			if _, err := writeString(w, s[last:i]); err != nil {
				return err
			}
		}
		if _, err := writeString(w, repl); err != nil {
			return err
		}
		last = i + 1
	}
	if last < len(s) {
		_, err := writeString(w, s[last:])
		return err
	}
	return nil
}

func writeEscapedBytes(w io.Writer, b []byte) error {
	first := bytes.IndexAny(b, "&<>\"'")
	if first < 0 {
		_, err := w.Write(b)
		return err
	}
	last := 0
	for i := first; i < len(b); i++ {
		repl := ""
		switch b[i] {
		case '&':
			repl = "&amp;"
		case '<':
			repl = "&lt;"
		case '>':
			repl = "&gt;"
		case '"':
			repl = "&#34;"
		case '\'':
			repl = "&#39;"
		}
		if repl == "" {
			continue
		}
		if last < i {
			if _, err := w.Write(b[last:i]); err != nil {
				return err
			}
		}
		if _, err := writeString(w, repl); err != nil {
			return err
		}
		last = i + 1
	}
	if last < len(b) {
		_, err := w.Write(b[last:])
		return err
	}
	return nil
}

func writeAttrPrefix(w io.Writer, name string) error {
	if err := WriteString(w, " "); err != nil {
		return err
	}
	if err := WriteString(w, name); err != nil {
		return err
	}
	return WriteString(w, `="`)
}

func writeAttrSuffix(w io.Writer) error {
	return WriteString(w, `"`)
}

func writeString(w io.Writer, s string) (int, error) {
	if sw, ok := w.(io.StringWriter); ok {
		return sw.WriteString(s)
	}
	if s == "" {
		return 0, nil
	}
	// io.Writer implementations must not mutate p, so this avoids the []byte(s) allocation.
	return w.Write(unsafe.Slice(unsafe.StringData(s), len(s)))
}

func stringify(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	case HTML:
		return string(x), true
	case fmt.Stringer:
		return x.String(), true
	case int:
		return strconv.Itoa(x), true
	case int8:
		return strconv.FormatInt(int64(x), 10), true
	case int16:
		return strconv.FormatInt(int64(x), 10), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case uint:
		return strconv.FormatUint(uint64(x), 10), true
	case uint8:
		return strconv.FormatUint(uint64(x), 10), true
	case uint16:
		return strconv.FormatUint(uint64(x), 10), true
	case uint32:
		return strconv.FormatUint(uint64(x), 10), true
	case uint64:
		return strconv.FormatUint(x, 10), true
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	}
	return "", false
}
