package gosmt

import "fmt"

const (
	RESULT_ERROR   = 0
	RESULT_SAT     = 1
	RESULT_UNSAT   = 2
	RESULT_UNKNOWN = 3
)

type solverBackend interface {
	clone() solverBackend
	check(query *BoolExprPtr) int
	model() map[string]*BVConst
	evalUpto(bv *BVExprPtr, pi *BoolExprPtr, n int) []*BVConst
}

type Solver struct {
	eb              *ExprBuilder
	backend         solverBackend
	constraints     map[uintptr]*BoolExprPtr
	symToContraints map[uintptr]map[uintptr]*BoolExprPtr
	symDependencies map[uintptr]map[uintptr]*BVExprPtr

	// A cache for previous evaluations
	model map[string]*BVConst
}

func NewZ3Solver(eb *ExprBuilder) *Solver {
	return &Solver{
		eb:              eb,
		backend:         newZ3Backend(),
		constraints:     make(map[uintptr]*BoolExprPtr),
		symToContraints: make(map[uintptr]map[uintptr]*BoolExprPtr),
		symDependencies: make(map[uintptr]map[uintptr]*BVExprPtr),
		model:           make(map[string]*BVConst),
	}
}

func (s *Solver) Clone() *Solver {
	clone := &Solver{
		eb:              s.eb,
		backend:         s.backend.clone(),
		constraints:     make(map[uintptr]*BoolExprPtr),
		symToContraints: make(map[uintptr]map[uintptr]*BoolExprPtr),
		symDependencies: make(map[uintptr]map[uintptr]*BVExprPtr),
		model:           make(map[string]*BVConst),
	}
	for k, val := range s.constraints {
		clone.constraints[k] = val
	}
	for k, val := range s.model {
		clone.model[k] = val
	}
	for k1, val1 := range s.symToContraints {
		set := make(map[uintptr]*BoolExprPtr)
		for k2, val2 := range val1 {
			set[k2] = val2
		}
		clone.symToContraints[k1] = set
	}
	for k1, val1 := range s.symDependencies {
		set := make(map[uintptr]*BVExprPtr)
		for k2, val2 := range val1 {
			set[k2] = val2
		}
		clone.symDependencies[k1] = set
	}
	return clone
}

func (s *Solver) registerConstraintForSym(sym *BVExprPtr, constraint *BoolExprPtr) {
	if _, ok := s.symToContraints[sym.Id()]; !ok {
		s.symToContraints[sym.Id()] = make(map[uintptr]*BoolExprPtr)
	}
	s.symToContraints[sym.Id()][constraint.Id()] = constraint
}

func (s *Solver) registerSymDepencency(sym1 *BVExprPtr, sym2 *BVExprPtr) {
	if _, ok := s.symDependencies[sym1.Id()]; !ok {
		s.symDependencies[sym1.Id()] = make(map[uintptr]*BVExprPtr)
	}
	if _, ok := s.symDependencies[sym2.Id()]; !ok {
		s.symDependencies[sym2.Id()] = make(map[uintptr]*BVExprPtr)
	}
	s.symDependencies[sym1.Id()][sym2.Id()] = sym2
	s.symDependencies[sym2.Id()][sym1.Id()] = sym1
}

func (s *Solver) getDependentConstraints(constraint ExprPtr) []*BoolExprPtr {
	// return all the constraints that are related with the input one (even indirectly)
	syms := s.eb.InvolvedInputs(constraint)
	symsMap := make(map[uintptr]*BVExprPtr)
	for i := 0; i < len(syms); i++ {
		symsMap[syms[i].Id()] = syms[i]
		otherSyms := s.symDependencies[syms[i].Id()]
		for _, osym := range otherSyms {
			symsMap[osym.Id()] = osym
		}
	}

	constraints := make(map[uintptr]*BoolExprPtr)
	for _, sym := range symsMap {
		if _, ok := s.symToContraints[sym.Id()]; !ok {
			continue
		}
		symConstraints := s.symToContraints[sym.Id()]
		for _, v := range symConstraints {
			constraints[v.Id()] = v
		}
	}

	res := make([]*BoolExprPtr, 0)
	for _, c := range constraints {
		res = append(res, c)
	}
	return res
}

func (s *Solver) Add(constraint *BoolExprPtr) {
	if _, ok := s.constraints[constraint.Id()]; ok {
		return
	}
	if constraint.IsConst() {
		c, _ := constraint.GetConst()
		if c {
			return
		}
	}
	s.constraints[constraint.Id()] = constraint

	syms := s.eb.InvolvedInputs(constraint)
	for i := 0; i < len(syms); i++ {
		sym := syms[i]
		s.registerConstraintForSym(sym, constraint)
		for j := i + 1; j < len(syms); j++ {
			s.registerSymDepencency(sym, syms[j])
		}
	}
}

func (s *Solver) Pi() *BoolExprPtr {
	res := s.eb.BoolVal(true)
	for _, val := range s.constraints {
		var err error
		res, err = s.eb.BoolAnd(res, val)
		if err != nil {
			// if it happens, we have a malformed path constraint
			panic(err)
		}
	}
	return res
}

func (s *Solver) pi(e ExprPtr) *BoolExprPtr {
	constraints := s.getDependentConstraints(e)
	res := s.eb.BoolVal(true)
	for _, v := range constraints {
		var err error
		res, err = s.eb.BoolAnd(res, v)
		if err != nil {
			panic(err)
		}
	}
	return res
}

func (s *Solver) checkSatCurrentModel(q *BoolExprPtr) int {
	if q.IsConst() {
		qVal, _ := q.GetConst()
		if qVal {
			return RESULT_SAT
		}
		return RESULT_UNSAT
	}

	evalQ := s.eb.eval(q, s.model)
	if evalQ.getInternal().kind() == TY_BOOL_CONST {
		evalQInt := evalQ.getInternal().(*internalBoolVal)
		if evalQInt.Value.Value {
			return RESULT_SAT
		}
	}
	return RESULT_UNKNOWN
}

func (s *Solver) Satisfiable() (int, error) {
	pi := s.Pi()
	satCurrentModel := s.checkSatCurrentModel(pi)
	if satCurrentModel == RESULT_SAT {
		return RESULT_SAT, nil
	}
	if satCurrentModel == RESULT_UNSAT {
		return RESULT_ERROR, fmt.Errorf("unsat state")
	}

	r := s.backend.check(s.Pi())
	// save the model
	s.model = s.backend.model()
	return r, nil
}

func (s *Solver) CheckSat(query *BoolExprPtr) int {
	pi, err := s.eb.BoolAnd(s.pi(query), query)
	if err != nil {
		panic(err)
	}
	satCurrentModel := s.checkSatCurrentModel(pi)
	if satCurrentModel == RESULT_UNKNOWN {
		return s.backend.check(pi)
	}
	return satCurrentModel
}

func (s *Solver) CheckSatAndAddIfSat(query *BoolExprPtr) int {
	pi, err := s.eb.BoolAnd(s.pi(query), query)
	if err != nil {
		panic(err)
	}
	result := s.checkSatCurrentModel(pi)
	if result == RESULT_UNKNOWN {
		result = s.backend.check(pi)
	}
	if result == RESULT_SAT {
		s.model = s.backend.model()
		s.Add(query)
	}
	return result
}

func (s *Solver) Model() map[string]*BVConst {
	return s.backend.model()
}

func (s *Solver) Eval(bv *BVExprPtr) *BVConst {
	bvEval := s.eb.eval(bv, s.model)
	if bvEval.getInternal().kind() == TY_CONST {
		bvEvalInt := bvEval.getInternal().(*internalBVV)
		return bvEvalInt.Value.Copy()
	}

	pi := s.pi(bv)
	res := s.backend.evalUpto(bv, pi, 1)
	if len(res) == 0 {
		return nil
	}
	s.model = s.backend.model()
	return res[0]
}

func (s *Solver) EvalList(bvs []*BVExprPtr) []*BVConst {
	if len(bvs) == 0 {
		return make([]*BVConst, 0)
	}

	joint := bvs[0]
	for _, e := range bvs[1:] {
		var err error
		joint, err = s.eb.Concat(e, joint)
		if err != nil {
			panic(err)
		}
	}

	jointVal := s.Eval(joint)
	pieces := make([]*BVConst, 0)
	accumulator := uint(0)
	for i := 0; i < len(bvs); i++ {
		pieces = append(pieces, jointVal.Slice(accumulator+bvs[i].Size()-1, accumulator))
		accumulator += bvs[i].Size()
	}
	return pieces
}

func (s *Solver) EvalUpto(bv *BVExprPtr, n int) []*BVConst {
	pi := s.pi(bv)
	r := s.backend.evalUpto(bv, pi, n)
	if len(r) > 0 {
		s.model = s.backend.model()
	}
	return r
}
