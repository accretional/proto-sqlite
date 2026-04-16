package main

import (
	"github.com/accretional/gluon/v2/compiler"
	pb "github.com/accretional/gluon/v2/pb"
)

// scalarizeX walks the AST and replaces every terminal with value "x"
// by a scalar node. The EBNF uses `"x"` as a placeholder for "a
// user-provided string" in productions like `name`, `string_literal`,
// and `blob_literal`; without this rewrite the compiler would emit
// `XKeyword x_keyword` fields that can only ever carry the literal
// character `x`. Scalars lower to proto3 `string` fields and let
// callers populate real identifiers at runtime.
//
// The input is not mutated; a deep copy is returned.
func scalarizeX(root *pb.ASTNode) *pb.ASTNode {
	if root == nil {
		return nil
	}
	if root.GetKind() == compiler.KindTerminal && root.GetValue() == "x" {
		return &pb.ASTNode{Kind: compiler.KindScalar, Value: "value"}
	}
	kids := make([]*pb.ASTNode, 0, len(root.GetChildren()))
	for _, c := range root.GetChildren() {
		kids = append(kids, scalarizeX(c))
	}
	return &pb.ASTNode{
		Kind:     root.GetKind(),
		Value:    root.GetValue(),
		Children: kids,
	}
}
