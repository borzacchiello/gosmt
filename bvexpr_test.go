package gosmt

import (
	"testing"
)

func TestAdd(t *testing.T) {
	sym1 := mkBVS("a", 32)
	sym2 := mkBVS("b", 32)

	children := make([]BVExpr, 0)
	children = append(children, sym1)
	children = append(children, sym2)
	children = append(children, mkBVV(42, 32))
	e, err := mkBVExprAdd(children)
	if err != nil {
		t.Error(err)
	}

	if e.String() != "a + b + 0x2a" {
		t.Error("invalid expression")
	}
}

func TestArithmetic(t *testing.T) {
	sym1 := mkBVS("a", 32)
	sym2 := mkBVS("b", 32)

	cc1 := make([]BVExpr, 0)
	cc1 = append(cc1, sym1)
	cc1 = append(cc1, sym2)
	cc1 = append(cc1, mkBVV(42, 32))
	e1, err := mkBVExprMul(cc1)
	if err != nil {
		t.Error(err)
	}

	cc2 := make([]BVExpr, 0)
	cc2 = append(cc2, e1)
	cc2 = append(cc2, mkBVV(12, 32))
	e2, err := mkBVExprAdd(cc2)
	if err != nil {
		t.Error(err)
	}

	cc3 := make([]BVExpr, 0)
	cc3 = append(cc3, mkBVV(0xfff00fff, 32))
	cc3 = append(cc3, e2)
	e3, err := mkBVExprAnd(cc3)
	if err != nil {
		t.Error(err)
	}

	cc4 := make([]BVExpr, 0)
	cc4 = append(cc4, e3)
	cc4 = append(cc4, mkBVV(15, 32))
	e4, err := mkBVExprOr(cc4)
	if err != nil {
		t.Error(err)
	}

	if e4.String() != "(0xfff00fff & ((a * b * 0x2a) + 0xc)) | 0xf" {
		t.Error("invalid expression")
	}
}
