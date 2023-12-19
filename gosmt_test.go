package gosmt

import (
	"fmt"
	"testing"
)

func TestBV(t *testing.T) {
	bv := MakeBV(-1294871, 32)
	if bv.String() != "<BV32 0xffec3de9>" {
		t.Errorf("incorrect BV")
	}
}

func TestBVAdd(t *testing.T) {
	bv1 := MakeBV(-10, 32)
	bv2 := MakeBV(128, 32)
	bv1.Add(bv2)

	if bv1.AsULong() != 118 {
		t.Errorf("incorrect BV")
	}
}

func TestBVSub(t *testing.T) {
	bv1 := MakeBV(-10, 32)
	bv2 := MakeBV(128, 32)
	bv1.Sub(bv2)

	if bv1.AsLong() != -138 {
		t.Errorf("incorrect BV")
	}
}

func TestSExt(t *testing.T) {
	bv := MakeBV(-10, 32)
	bv.SExt(32)

	if bv.Size != 64 || bv.AsLong() != -10 {
		t.Errorf("incorrect BV")
	}
}

func TestNonstandardSizes(t *testing.T) {
	bv := MakeBV(1, 3)
	bv.Add(MakeBV(7, 3))
	if bv.AsULong() != 0 {
		t.Errorf("incorrect BV")
	}
}

func TestWrongSizes(t *testing.T) {
	err := MakeBV(1, 3).Add(MakeBV(1, 4))
	if err == nil {
		t.Errorf("should return an error")
	}
}

func TestTruncateConcat(t *testing.T) {
	bv := MakeBV(42, 8)
	bv.Concat(MakeBV(43, 8))
	bv.Concat(MakeBV(44, 8))
	bv.Concat(MakeBV(45, 8))

	fmt.Printf("bv: %s\n", bv)

	b := bv.Copy()
	b.Truncate(7, 0)
	if b.AsULong() != 45 {
		t.Errorf("incorrect BV")
	}

	b = bv.Copy()
	b.Truncate(15, 8)
	if b.AsULong() != 44 {
		t.Errorf("incorrect BV")
	}
}

func TestSlice(t *testing.T) {
	bv := MakeBV(0xdeadbeef, 32)

	if bv.Slice(7, 0).AsULong() != 0xef {
		t.Errorf("incorrect BV")
	}
	if bv.Slice(15, 8).AsULong() != 0xbe {
		t.Errorf("incorrect BV")
	}
	if bv.Slice(23, 16).AsULong() != 0xad {
		t.Errorf("incorrect BV")
	}
	if bv.Slice(32, 24).AsULong() != 0xde {
		t.Errorf("incorrect BV")
	}
}

func TestAShr(t *testing.T) {
	bv := MakeBV(-1, 32)
	bv.AShr(13)

	if bv.AsLong() != -1 {
		t.Errorf("incorrect BV")
	}

	bv = MakeBV(-2, 32)
	bv.AShr(1)

	if bv.AsLong() != -1 {
		t.Errorf("incorrect BV")
	}
}

func TestNeg(t *testing.T) {
	bv := MakeBV(-42, 18)

	bv.Neg()
	if bv.AsLong() != 42 {
		t.Errorf("incorrect BV")
	}
	bv.Neg()
	if bv.AsLong() != -42 {
		t.Errorf("incorrect BV")
	}
}

func TestCmp(t *testing.T) {
	bv1 := MakeBV(-10, 32)
	bv2 := MakeBV(-11, 32)
	bv3 := MakeBV(1, 32)

	v, err := bv1.SGt(bv2)
	if err != nil || !v.Value {
		t.Errorf("[%s s> %s = %s] incorrect SGt result", bv1, bv2, v)
	}

	v, err = bv1.SGe(bv2)
	if err != nil || !v.Value {
		t.Errorf("[%s s>= %s = %s] incorrect SGe result", bv1, bv2, v)
	}

	v, err = bv1.SLt(bv2)
	if err != nil || v.Value {
		t.Errorf("[%s s< %s = %s] incorrect SLt result", bv1, bv2, v)
	}

	v, err = bv1.SLe(bv2)
	if err != nil || v.Value {
		t.Errorf("[%s s<= %s = %s] incorrect SLe result", bv1, bv2, v)
	}

	v, err = bv1.Ult(bv3)
	if err != nil || v.Value {
		t.Errorf("[%s u< %s = %s] incorrect Ult result", bv1, bv2, v)
	}
}
