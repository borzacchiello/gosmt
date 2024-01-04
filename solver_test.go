package gosmt

import (
	"testing"
)

func TestSolverSat1(t *testing.T) {
	s := NewZ3Solver()

	a := s.Builder.BVS("a", 32)
	e, _ := s.Builder.Ule(a, s.Builder.BVV(42, 32))
	s.Add(e)

	e, _ = s.Builder.UGe(a, s.Builder.BVV(21, 32))
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
	s := NewZ3Solver()

	a := s.Builder.BVS("a", 32)
	e, _ := s.Builder.Ule(a, s.Builder.BVV(42, 32))
	s.Add(e)

	e, _ = s.Builder.UGe(a, s.Builder.BVV(21, 32))
	s.Add(e)

	aVal := s.Eval(a).AsULong()
	if aVal > 42 || aVal < 21 {
		t.Error("invalid eval value")
		return
	}
}

func TestSolverEval2(t *testing.T) {
	s := NewZ3Solver()

	a := s.Builder.BVS("a", 32)
	e, _ := s.Builder.Ule(a, s.Builder.BVV(42, 32))
	s.Add(e)

	e, _ = s.Builder.UGe(a, s.Builder.BVV(21, 32))
	s.Add(e)

	vals := s.EvalUpto(a, 128)
	if len(vals) != 42-21+1 {
		t.Error("unable to find all values")
		return
	}
}
