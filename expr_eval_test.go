package gosmt

import (
	"testing"
)

func TestEval1(t *testing.T) {
	eb := NewExprBuilder()
	a := eb.BVS("a", 32)
	b := eb.BVS("b", 32)

	interpr := make(map[string]*BVConst)
	interpr["a"] = MakeBVConst(42, 32)

	e, _ := eb.Add(a, b)
	evaluated := eb.eval(e, interpr)
	if evaluated.getInternal().String() != "b + 0x2a" {
		t.Error("invalid eval")
	}
}
