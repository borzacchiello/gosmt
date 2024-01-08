package gosmt_test

import (
	"runtime"
	"testing"

	"github.com/borzacchiello/gosmt"
)

func isErr(t *testing.T, err error) bool {
	if err != nil {
		t.Error(err)
		return true
	}
	return false
}

func getByte(t *testing.T, eb *gosmt.ExprBuilder, expr *gosmt.BVExprPtr, i uint) *gosmt.BVExprPtr {
	b, err := eb.Extract(expr, (i+1)*8-1, i*8)
	if isErr(t, err) {
		return nil
	}
	return b
}

func TestCache1(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	var oldid uintptr
	{
		s1 := eb.BVS("s1", 32)
		s2 := eb.BVS("s2", 32)
		e, err := eb.Add(s1, s2)
		if err != nil {
			t.Error(err)
			return
		}

		ss1 := eb.BVS("s1", 32)
		if s1.Id() != ss1.Id() {
			t.Error("should be the same object")
			return
		}
		ee, _ := eb.Add(ss1, s2)
		if e.Id() != ee.Id() {
			t.Error("should be the same object")
			return
		}
		oldid = s1.Id()
	}

	runtime.GC()

	for i := 0; i < 32; i++ {
		// create noise...
		eb.BVV(int64(i), 32)
	}

	runtime.GC()

	s1 := eb.BVS("s1", 32)
	if s1.Id() == oldid {
		t.Error("should not be the same object")
		return
	}
}

func TestCache2(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	s1 := eb.BVS("s1", 32)
	var oldid uintptr
	{
		s2 := eb.BVS("s2", 32)
		e, err := eb.Add(s1, s2)
		if err != nil {
			t.Error(err)
			return
		}

		ss1 := eb.BVS("s1", 32)
		if s1.Id() != ss1.Id() {
			t.Error("should be the same object")
			return
		}
		ee, _ := eb.Add(ss1, s2)
		if e.Id() != ee.Id() {
			t.Error("should be the same object")
			return
		}

		oldid = s2.Id()
	}

	runtime.GC()

	for i := 0; i < 32; i++ {
		// create noise...
		eb.BVV(int64(i), 32)
	}

	runtime.GC()

	s1_cpy := eb.BVS("s1", 32)
	if s1.Id() != s1_cpy.Id() {
		t.Error("should be the same object")
		return
	}
	s2_cpy := eb.BVS("s2", 32)
	if oldid == s2_cpy.Id() {
		t.Error("should not be the same object")
		return
	}
}

func TestCache3(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	var addExpr *gosmt.BVExprPtr
	var oldid1, oldid2 uintptr
	var addExprId uintptr
	{
		s1 := eb.BVS("s1", 32)
		s2 := eb.BVS("s2", 32)
		e, err := eb.Add(s1, s2)
		if err != nil {
			t.Error(err)
			return
		}

		ss1 := eb.BVS("s1", 32)
		if s1.Id() != ss1.Id() {
			t.Error("should be the same object")
			return
		}
		e_cpy, _ := eb.Add(ss1, s2)
		if e.Id() != e_cpy.Id() {
			t.Error("should be the same object")
			return
		}

		oldid1 = s1.Id()
		oldid2 = s2.Id()

		addExpr = e_cpy
		addExprId = addExpr.Id()
	}

	runtime.GC()

	for i := 0; i < 32; i++ {
		// create noise...
		eb.BVV(int64(i), 32)
	}

	runtime.GC()

	s1_cpy := eb.BVS("s1", 32)
	if oldid1 != s1_cpy.Id() {
		t.Error("should be the same object")
		return
	}
	s2_cpy := eb.BVS("s2", 32)
	if oldid2 != s2_cpy.Id() {
		t.Error("should be the same object")
		return
	}

	if addExpr.Id() != addExprId {
		t.Error("wrong id")
		return
	}
}

func TestCache4(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	v1 := eb.BVV(12, 32)
	v2 := eb.BVV(12, 32)

	if v1.Id() != v2.Id() {
		t.Error("should be the same object")
		return
	}
}

func TestAdd1(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	a := eb.BVS("a", 64)
	b := eb.BVS("b", 64)
	e, _ := eb.Add(a, eb.Neg(b))
	e, _ = eb.Add(e, eb.Neg(e))

	if e.String() != "0x0" {
		t.Error("failed Add simplification")
		return
	}
}

func TestShift1(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	sym := eb.BVS("sym", 64)
	e, err := eb.AShr(sym, eb.BVV(16, 64))
	if err != nil {
		t.Error(err)
		return
	}
	e, err = eb.Shl(e, eb.BVV(8, 64))
	if err != nil {
		t.Error(err)
		return
	}

	if e.String() != "(sym a>> 0x10) << 0x8" {
		t.Error("unexpected expression")
		return
	}
}

func TestBool1(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	a, err := eb.Eq(eb.BVS("a", 1), eb.BVV(1, 1))
	if err != nil {
		t.Error(err)
		return
	}
	b, err := eb.Eq(eb.BVS("b", 1), eb.BVV(1, 1))
	if err != nil {
		t.Error(err)
		return
	}

	e, err := eb.BoolAnd(a, b)
	if err != nil {
		t.Error(err)
		return
	}
	e, err = eb.BoolNot(e)
	if err != nil {
		t.Error(err)
		return
	}
	e, err = eb.BoolAnd(e, eb.BoolVal(true))
	if err != nil {
		t.Error(err)
		return
	}
	e, err = eb.BoolOr(e, eb.BoolVal(false))
	if err != nil {
		t.Error(err)
		return
	}
	if e.String() != "(!(a == 0x1)) || (!(b == 0x1))" {
		t.Error("unexpected expression")
		return
	}
}

func TestBVCompare(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	a := eb.BVS("a", 64)
	b := eb.BVS("b", 64)
	e, _ := eb.Ule(a, b)
	if e.String() != "a u<= b" {
		t.Error("invalid expression")
		return
	}
}

func TestConcat1(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	a := eb.BVS("a", 32)
	p1 := getByte(t, eb, a, 0)
	p2 := getByte(t, eb, a, 1)
	p3 := getByte(t, eb, a, 2)
	p4 := getByte(t, eb, a, 3)

	if p1 == nil || p2 == nil || p3 == nil || p4 == nil {
		return
	}

	c, err := eb.Concat(p4, p3)
	if isErr(t, err) {
		return
	}
	c, err = eb.Concat(c, p2)
	if isErr(t, err) {
		return
	}
	c, err = eb.Concat(c, p1)
	if isErr(t, err) {
		return
	}

	if c.String() != "a" {
		t.Error("Unable to simplify concat")
		return
	}
}

func TestInvolvedInputs(t *testing.T) {
	eb := gosmt.NewExprBuilder()

	a := eb.BVS("a", 8)
	b := eb.BVS("b", 8)

	e, _ := eb.Add(a, b)
	e, _ = eb.Xor(e, a)
	e, _ = eb.And(e, eb.BVV(1337, 8))

	inputs := eb.InvolvedInputs(e)
	if len(inputs) != 2 {
		t.Error("expecting two inputs")
		return
	}
	if inputs[0].Id() != a.Id() && inputs[1].Id() != a.Id() {
		t.Error("a not found")
	}
	if inputs[0].Id() != b.Id() && inputs[1].Id() != b.Id() {
		t.Error("b not found")
	}
}
