package sqliteembed

import (
	"testing"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

// digit builds the Digit production for a single decimal digit 0–9.
func digit(d int) *sqlitepb.Digit {
	switch d {
	case 0:
		return &sqlitepb.Digit{Value: &sqlitepb.Digit_DigitZeroKeyword{DigitZeroKeyword: &sqlitepb.DigitZeroKeyword{}}}
	case 1:
		return &sqlitepb.Digit{Value: &sqlitepb.Digit_DigitOneKeyword{DigitOneKeyword: &sqlitepb.DigitOneKeyword{}}}
	case 2:
		return &sqlitepb.Digit{Value: &sqlitepb.Digit_DigitTwoKeyword{DigitTwoKeyword: &sqlitepb.DigitTwoKeyword{}}}
	case 4:
		return &sqlitepb.Digit{Value: &sqlitepb.Digit_DigitFourKeyword{DigitFourKeyword: &sqlitepb.DigitFourKeyword{}}}
	default: // only the digits used below are needed
		return &sqlitepb.Digit{Value: &sqlitepb.Digit_DigitThreeKeyword{DigitThreeKeyword: &sqlitepb.DigitThreeKeyword{}}}
	}
}

// TestRender_NumericLiteralMultiDigit pins the fix for the digits bug: a
// NumericLiteral is decomposed into per-digit productions, and the renderer
// must emit the whole literal as ONE space-free token rather than "1 0 2 4".
func TestRender_NumericLiteralMultiDigit(t *testing.T) {
	// 1024
	got, err := RenderSQL(&sqlitepb.NumericLiteral{
		Digit:   digit(1),
		Digit_2: []*sqlitepb.Digit{digit(0), digit(2), digit(4)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "1024" {
		t.Errorf("got %q, want %q", got, "1024")
	}

	// 3.14
	got, err = RenderSQL(&sqlitepb.NumericLiteral{
		Digit: digit(3),
		FullStop: &sqlitepb.NumericLiteral_FullStop{
			Digit: []*sqlitepb.Digit{digit(1), digit(4)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "3.14" {
		t.Errorf("got %q, want %q", got, "3.14")
	}

	// 1E10  (exponent: the "E" and the exponent digits must also stay attached)
	got, err = RenderSQL(&sqlitepb.NumericLiteral{
		Digit: digit(1),
		E:     &sqlitepb.NumericLiteral_E{Digit: digit(1), Digit_2: []*sqlitepb.Digit{digit(0)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "1E10" {
		t.Errorf("got %q, want %q", got, "1E10")
	}
}
