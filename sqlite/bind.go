package sqliteembed

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

// bindParams replaces each ? in sql (that is not inside a '...' literal)
// with the SQL-escaped form of the corresponding param, left-to-right.
// Returns an error if the ? count does not match len(params).
func bindParams(sql string, params []*sqlitepb.Value) (string, error) {
	var out strings.Builder
	paramIdx := 0
	i := 0
	n := len(sql)

	for i < n {
		ch := sql[i]

		if ch == '\'' {
			// copy the string literal verbatim, handling '' escapes
			out.WriteByte(ch)
			i++
			for i < n {
				c := sql[i]
				out.WriteByte(c)
				i++
				if c == '\'' {
					if i < n && sql[i] == '\'' {
						// escaped quote — copy and continue inside literal
						out.WriteByte(sql[i])
						i++
					} else {
						break // end of literal
					}
				}
			}
			continue
		}

		if ch == '?' {
			if paramIdx >= len(params) {
				return "", fmt.Errorf("more ? placeholders than params (got %d)", len(params))
			}
			lit, err := escapeSQLValue(params[paramIdx])
			if err != nil {
				return "", fmt.Errorf("param %d: %w", paramIdx, err)
			}
			out.WriteString(lit)
			paramIdx++
			i++
			continue
		}

		out.WriteByte(ch)
		i++
	}

	if paramIdx != len(params) {
		return "", fmt.Errorf("param count mismatch: %d ? placeholders, %d params", paramIdx, len(params))
	}
	return out.String(), nil
}

// escapeSQLValue returns the SQL literal representation of v.
func escapeSQLValue(v *sqlitepb.Value) (string, error) {
	switch x := v.GetV().(type) {
	case *sqlitepb.Value_Text:
		return "'" + strings.ReplaceAll(x.Text, "'", "''") + "'", nil
	case *sqlitepb.Value_Integer:
		return strconv.FormatInt(x.Integer, 10), nil
	case *sqlitepb.Value_Real:
		return strconv.FormatFloat(x.Real, 'g', -1, 64), nil
	case *sqlitepb.Value_Blob:
		return "x'" + hex.EncodeToString(x.Blob) + "'", nil
	case *sqlitepb.Value_Null:
		return "NULL", nil
	default:
		return "", fmt.Errorf("Value has no v set")
	}
}
