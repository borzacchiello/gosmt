package gosmt

import (
	"testing"
)

func TestAdd(t *testing.T) {
	sym1 := mkinternalBVS("a", 32)
	sym2 := mkinternalBVS("b", 32)

	children := make([]*BVExprPtr, 0)
	children = append(children, wrapBVExpr(sym1))
	children = append(children, wrapBVExpr(sym2))
	children = append(children, wrapBVExpr(mkinternalBVV(42, 32)))
	e, err := mkinternalBVExprAdd(children)
	if err != nil {
		t.Error(err)
		return
	}

	if e.String() != "a + b + 0x2a" {
		t.Error("invalid expression")
		return
	}
}

func TestArithmetic(t *testing.T) {
	sym1 := mkinternalBVS("a", 32)
	sym2 := mkinternalBVS("b", 32)

	cc1 := make([]*BVExprPtr, 0)
	cc1 = append(cc1, wrapBVExpr(sym1))
	cc1 = append(cc1, wrapBVExpr(sym2))
	cc1 = append(cc1, wrapBVExpr(mkinternalBVV(42, 32)))
	e1, err := mkinternalBVExprMul(cc1)
	if err != nil {
		t.Error(err)
		return
	}

	cc2 := make([]*BVExprPtr, 0)
	cc2 = append(cc2, wrapBVExpr(e1))
	cc2 = append(cc2, wrapBVExpr(mkinternalBVV(12, 32)))
	e2, err := mkinternalBVExprAdd(cc2)
	if err != nil {
		t.Error(err)
		return
	}

	cc3 := make([]*BVExprPtr, 0)
	cc3 = append(cc3, wrapBVExpr(mkinternalBVV(0xfff00fff, 32)))
	cc3 = append(cc3, wrapBVExpr(e2))
	e3, err := mkinternalBVExprAnd(cc3)
	if err != nil {
		t.Error(err)
		return
	}

	cc4 := make([]*BVExprPtr, 0)
	cc4 = append(cc4, wrapBVExpr(e3))
	cc4 = append(cc4, wrapBVExpr(mkinternalBVV(15, 32)))
	e4, err := mkinternalBVExprOr(cc4)
	if err != nil {
		t.Error(err)
		return
	}

	if e4.String() != "(0xfff00fff & ((a * b * 0x2a) + 0xc)) | 0xf" {
		t.Error("invalid expression")
		return
	}
}

func TestBVCompare(t *testing.T) {
	eb := NewExprBuilder()

	a := eb.BVS("a", 64)
	b := eb.BVS("b", 64)
	e, _ := eb.Ule(a, b)
	if e.String() != "a u<= b" {
		t.Error("invalid expression")
		return
	}
}
