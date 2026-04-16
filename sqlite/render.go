package sqliteembed

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	sqlitepb "github.com/accretional/proto-sqlite/sqlite/pb"
)

// RenderSQL serializes a typed proto message (a SqlStmtList, a single
// SqlStmt, or any of the sub-messages) back into SQL text. It walks
// the message via reflection, emitting leading-keyword tokens recorded
// in sqlitepb.MessagePrefix and recursing into populated fields in
// proto declaration order.
//
// The renderer relies on the fact that every field in the generated
// schema is a message type (no scalars survived the grammar pipeline);
// keyword messages are empty markers whose FQN maps to their literal
// in MessagePrefix.
func RenderSQL(msg proto.Message) (string, error) {
	var toks []string
	if err := renderMessage(msg.ProtoReflect(), &toks); err != nil {
		return "", err
	}
	return joinTokens(toks), nil
}

func renderMessage(m protoreflect.Message, toks *[]string) error {
	fqn := "." + string(m.Descriptor().FullName())
	if prefix, ok := sqlitepb.MessagePrefix[fqn]; ok {
		*toks = append(*toks, prefix...)
	}

	fds := m.Descriptor().Fields()
	for i := 0; i < fds.Len(); i++ {
		fd := fds.Get(i)
		if !m.Has(fd) {
			continue
		}
		val := m.Get(fd)
		if fd.IsList() {
			list := val.List()
			for j := 0; j < list.Len(); j++ {
				if err := renderValue(fd, list.Get(j), toks); err != nil {
					return err
				}
			}
			continue
		}
		if err := renderValue(fd, val, toks); err != nil {
			return err
		}
	}
	return nil
}

func renderValue(fd protoreflect.FieldDescriptor, val protoreflect.Value, toks *[]string) error {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return renderMessage(val.Message(), toks)
	case protoreflect.StringKind:
		*toks = append(*toks, val.String())
		return nil
	default:
		return fmt.Errorf("unsupported field kind %v at %s", fd.Kind(), fd.FullName())
	}
}

// joinTokens stitches tokens into SQL text with conservative spacing:
// punctuation that conventionally hugs its neighbour ("(", ")", ",",
// ";", ".") is attached without a preceding or trailing space as
// appropriate. Everything else is separated by a single space.
func joinTokens(toks []string) string {
	var b strings.Builder
	for i, t := range toks {
		if i == 0 {
			b.WriteString(t)
			continue
		}
		prev := toks[i-1]
		if noSpaceBefore(t) || noSpaceAfter(prev) {
			b.WriteString(t)
			continue
		}
		b.WriteByte(' ')
		b.WriteString(t)
	}
	return b.String()
}

func noSpaceBefore(tok string) bool {
	switch tok {
	case ")", ",", ";", ".":
		return true
	}
	return false
}

func noSpaceAfter(tok string) bool {
	switch tok {
	case "(", ".":
		return true
	}
	return false
}
