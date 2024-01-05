package gosmt

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
}

func NewZ3Solver(eb *ExprBuilder) *Solver {
	return &Solver{
		eb:              eb,
		backend:         newZ3Backend(),
		constraints:     make(map[uintptr]*BoolExprPtr),
		symToContraints: make(map[uintptr]map[uintptr]*BoolExprPtr),
		symDependencies: make(map[uintptr]map[uintptr]*BVExprPtr),
	}
}

func (s *Solver) Clone() *Solver {
	clone := &Solver{
		eb:              s.eb,
		backend:         s.backend.clone(),
		constraints:     make(map[uintptr]*BoolExprPtr),
		symToContraints: make(map[uintptr]map[uintptr]*BoolExprPtr),
		symDependencies: make(map[uintptr]map[uintptr]*BVExprPtr),
	}
	for k, val := range s.constraints {
		clone.constraints[k] = val
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

func (s *Solver) Satisfiable() int {
	return s.backend.check(s.Pi())
}

func (s *Solver) CheckSat(query *BoolExprPtr) int {
	pi, err := s.eb.BoolAnd(s.pi(query), query)
	if err != nil {
		panic(err)
	}
	return s.backend.check(pi)
}

func (s *Solver) Model() map[string]*BVConst {
	return s.backend.model()
}

func (s *Solver) Eval(bv *BVExprPtr) *BVConst {
	pi := s.pi(bv)
	res := s.backend.evalUpto(bv, pi, 1)
	if len(res) == 0 {
		return nil
	}
	return res[0]
}

func (s *Solver) EvalUpto(bv *BVExprPtr, n int) []*BVConst {
	pi := s.pi(bv)
	return s.backend.evalUpto(bv, pi, n)
}
