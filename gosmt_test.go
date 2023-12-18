package gosmt

import (
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
