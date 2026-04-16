package sqliteembed

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// parseQuoteLine parses one value line from sqlite3's ".mode quote" output.
// Returns parallel slices: cells ([]byte per cell) and nulls (true where
// the cell is SQL NULL). Text cells return their string content as bytes;
// blob cells return decoded binary; number cells return their literal as
// bytes; NULL cells return nil with nulls[i]==true.
func parseQuoteLine(line string) (cells [][]byte, nulls []bool, err error) {
	i := 0
	n := len(line)

	emitCell := func(val []byte, isNull bool) {
		cells = append(cells, val)
		nulls = append(nulls, isNull)
	}

	for {
		if i >= n {
			// empty trailing cell after a comma
			emitCell([]byte{}, false)
			break
		}

		ch := line[i]

		switch {
		case ch == '\'':
			// text literal: scan until unescaped '
			i++ // skip opening quote
			var buf strings.Builder
			for {
				if i >= n {
					return nil, nil, fmt.Errorf("unterminated text literal")
				}
				c := line[i]
				if c == '\'' {
					if i+1 < n && line[i+1] == '\'' {
						buf.WriteByte('\'')
						i += 2
					} else {
						i++ // skip closing quote
						break
					}
				} else {
					buf.WriteByte(c)
					i++
				}
			}
			emitCell([]byte(buf.String()), false)

		case (ch == 'X' || ch == 'x') && i+1 < n && line[i+1] == '\'':
			// blob literal: X'hex...'
			i += 2 // skip X'
			start := i
			for i < n && line[i] != '\'' {
				i++
			}
			if i >= n {
				return nil, nil, fmt.Errorf("unterminated blob literal")
			}
			decoded, err2 := hex.DecodeString(line[start:i])
			if err2 != nil {
				return nil, nil, fmt.Errorf("decode blob hex: %w", err2)
			}
			i++ // skip closing quote
			emitCell(decoded, false)

		case ch == 'N' && strings.HasPrefix(line[i:], "NULL"):
			i += 4
			emitCell(nil, true)

		default:
			// bare integer or float
			start := i
			for i < n && line[i] != ',' {
				i++
			}
			emitCell([]byte(line[start:i]), false)
		}

		// after each cell expect comma or end of line
		if i >= n {
			break
		}
		if line[i] != ',' {
			return nil, nil, fmt.Errorf("expected ',' at pos %d, got %q", i, line[i])
		}
		i++ // skip comma
		if i >= n {
			// trailing comma → empty final cell
			emitCell([]byte{}, false)
			break
		}
	}

	return cells, nulls, nil
}
