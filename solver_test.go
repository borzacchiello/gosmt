package gosmt

import (
	"testing"
)

func TestSolverSat1(t *testing.T) {
	eb := NewExprBuilder()
	s := NewZ3Solver(eb)

	a := eb.BVS("a", 32)
	e, _ := eb.Ule(a, eb.BVV(42, 32))
	s.Add(e)

	e, _ = eb.UGe(a, eb.BVV(21, 32))
	sat := s.CheckSat(e)
	if sat != RESULT_SAT {
		t.Error("should be sat")
		return
	}

	m := s.Model()
	if _, ok := m["a"]; !ok {
		t.Error("unable to find the assignment")
		return
	}
}

func TestSolverEval1(t *testing.T) {
	eb := NewExprBuilder()
	s := NewZ3Solver(eb)

	a := eb.BVS("a", 32)
	e, _ := eb.Ule(a, eb.BVV(42, 32))
	s.Add(e)

	e, _ = eb.UGe(a, eb.BVV(21, 32))
	s.Add(e)

	aVal := s.Eval(a).AsULong()
	if aVal > 42 || aVal < 21 {
		t.Error("invalid eval value")
		return
	}
}

func TestSolverEval2(t *testing.T) {
	eb := NewExprBuilder()
	s := NewZ3Solver(eb)

	a := eb.BVS("a", 32)
	e, _ := eb.Ule(a, eb.BVV(42, 32))
	s.Add(e)

	e, _ = eb.UGe(a, eb.BVV(21, 32))
	s.Add(e)

	vals := s.EvalUpto(a, 128)
	if len(vals) != 42-21+1 {
		t.Error("unable to find all values")
		return
	}
}
