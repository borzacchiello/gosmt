package gosmt

import (
	"fmt"

	"github.com/aclements/go-z3/z3"
)

type z3backend struct {
	ctx    *z3.Context
	cfg    *z3.Config
	solver *z3.Solver

	lastSymbols map[uintptr]z3.BV
}

func newZ3Backend() *z3backend {
	cfg := z3.NewContextConfig()
	ctx := z3.NewContext(cfg)
	return &z3backend{
		ctx:    ctx,
		cfg:    cfg,
		solver: z3.NewSolver(ctx),
	}
}

func (s *z3backend) check(query *BoolExprPtr) int {
	s.solver.Reset()
	s.lastSymbols = make(map[uintptr]z3.BV)

	cache := make(map[uintptr]z3.Value)
	if query.Kind() == TY_BOOL_AND {
		andQuery := query.e.(*internalBoolExprNaryOp)
		for i := 0; i < len(andQuery.children); i++ {
			z3query := s.convert(andQuery.children[i].e, cache, s.lastSymbols)
			s.solver.Assert(z3query.(z3.Bool))
		}
	} else {
		z3query := s.convert(query.e, cache, s.lastSymbols)
		s.solver.Assert(z3query.(z3.Bool))
	}

	r, err := s.solver.Check()
	if err != nil {
		return RESULT_UNKNOWN
	}
	if r {
		return RESULT_SAT
	}
	return RESULT_UNSAT
}

func convertZ3Const(c z3.BV) (*BVConst, error) {
	v := MakeBVConstFromString(c.String()[2:], 16, uint(c.Sort().BVSize()))
	if v == nil {
		return nil, fmt.Errorf("not a constant")
	}
	return v, nil
}

func (s *z3backend) model() map[string]*BVConst {
	m := s.solver.Model()
	if m == nil {
		return nil
	}

	res := make(map[string]*BVConst)
	for _, sym := range s.lastSymbols {
		v := m.Eval(sym, false).(z3.BV)
		c, err := convertZ3Const(v)
		if err != nil {
			panic("unable to create constant")
		}
		res[sym.String()] = c
	}
	return res
}

func (s *z3backend) evalUpto(bv *BVExprPtr, pi *BoolExprPtr, n int) []*BVConst {
	s.solver.Reset()
	s.lastSymbols = make(map[uintptr]z3.BV, 0)
	cache := make(map[uintptr]z3.Value)

	values := make([]*BVConst, 0)
	bvZ3 := s.convert(bv.e, cache, s.lastSymbols).(z3.BV)
	if pi.Kind() == TY_BOOL_AND {
		andQuery := pi.e.(*internalBoolExprNaryOp)
		for i := 0; i < len(andQuery.children); i++ {
			z3query := s.convert(andQuery.children[i].e, cache, s.lastSymbols)
			s.solver.Assert(z3query.(z3.Bool))
		}
	} else {
		z3query := s.convert(pi.e, cache, s.lastSymbols)
		s.solver.Assert(z3query.(z3.Bool))
	}

	for {
		r, err := s.solver.Check()
		if err != nil || !r {
			break
		}

		m := s.solver.Model()
		if m == nil {
			panic("no model")
		}

		v := m.Eval(bvZ3, true).(z3.BV)
		c, err := convertZ3Const(v)
		if err != nil {
			panic("unable to convert constant to Z3")
		}
		values = append(values, c)
		s.solver.Assert(bvZ3.NE(v))

		n -= 1
		if n <= 0 {
			break
		}
	}
	return values
}

func (s *z3backend) convert(e internalExpr, cache map[uintptr]z3.Value, symbols map[uintptr]z3.BV) z3.Value {
	if v, ok := cache[e.rawPtr()]; ok {
		return v
	}

	var result z3.Value
	switch e.kind() {
	case TY_SYM:
		bv := e.(*internalBVS)
		result = s.ctx.BVConst(bv.name, int(bv.size()))
		symbols[bv.rawPtr()] = result.(z3.BV)
	case TY_CONST:
		bv := e.(*internalBVV)
		result = s.ctx.FromBigInt(bv.Value.value, s.ctx.BVSort(int(bv.size())))
	case TY_EXTRACT:
		e := e.(*internalBVExprExtract)
		child := s.convert(e.child.e, cache, symbols).(z3.BV)
		result = child.Extract(int(e.high), int(e.low))
	case TY_CONCAT:
		e := e.(*internalBVExprConcat)
		res := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.BV)
			res = res.Concat(child)
		}
		result = res
	case TY_ZEXT:
		e := e.(*internalBVExprExtend)
		child := s.convert(e, cache, symbols).(z3.BV)
		result = child.ZeroExtend(int(e.n))
	case TY_SEXT:
		e := e.(*internalBVExprExtend)
		child := s.convert(e, cache, symbols).(z3.BV)
		result = child.SignExtend(int(e.n))
	case TY_ITE:
		e := e.(*internalBVExprITE)
		guard := s.convert(e.cond.e, cache, symbols).(z3.Bool)
		iftrue := s.convert(e.iftrue.e, cache, symbols).(z3.BV)
		iffalse := s.convert(e.iffalse.e, cache, symbols).(z3.BV)
		result = guard.IfThenElse(iftrue, iffalse)
	case TY_NOT:
		e := e.(*internalBVExprUnArithmetic)
		child := s.convert(e, cache, symbols).(z3.BV)
		result = child.Not()
	case TY_NEG:
		e := e.(*internalBVExprUnArithmetic)
		child := s.convert(e, cache, symbols).(z3.BV)
		result = child.Neg()
	case TY_SHL:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.Lsh(rhs)
	case TY_LSHR:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.URsh(rhs)
	case TY_ASHR:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.SRsh(rhs)
	case TY_AND:
		e := e.(*internalBVExprBinArithmetic)
		res := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.BV)
			res = res.And(child)
		}
		result = res
	case TY_OR:
		e := e.(*internalBVExprBinArithmetic)
		res := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.BV)
			res = res.Or(child)
		}
		result = res
	case TY_XOR:
		e := e.(*internalBVExprBinArithmetic)
		res := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.BV)
			res = res.Xor(child)
		}
		result = res
	case TY_ADD:
		e := e.(*internalBVExprBinArithmetic)
		res := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.BV)
			res = res.Add(child)
		}
		result = res
	case TY_MUL:
		e := e.(*internalBVExprBinArithmetic)
		res := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.BV)
			res = res.Mul(child)
		}
		result = res
	case TY_SDIV:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.SDiv(rhs)
	case TY_UDIV:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.UDiv(rhs)
	case TY_SREM:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.SRem(rhs)
	case TY_UREM:
		e := e.(*internalBVExprBinArithmetic)
		lhs := s.convert(e.children[0].e, cache, symbols).(z3.BV)
		rhs := s.convert(e.children[1].e, cache, symbols).(z3.BV)
		result = lhs.URem(rhs)
	case TY_ULT:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.ULT(rhs)
	case TY_ULE:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.ULE(rhs)
	case TY_UGT:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.UGT(rhs)
	case TY_UGE:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.UGE(rhs)
	case TY_SLT:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.SLT(rhs)
	case TY_SLE:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.SLE(rhs)
	case TY_SGT:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.SGT(rhs)
	case TY_SGE:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.SGE(rhs)
	case TY_EQ:
		e := e.(*internalBoolExprCmp)
		lhs := s.convert(e.lhs.e, cache, symbols).(z3.BV)
		rhs := s.convert(e.rhs.e, cache, symbols).(z3.BV)
		result = lhs.Eq(rhs)
	case TY_BOOL_CONST:
		e := e.(*internalBoolVal)
		result = s.ctx.FromBool(e.Value.Value)
	case TY_BOOL_NOT:
		e := e.(*internalBoolUnArithmetic)
		child := s.convert(e.child.e, cache, symbols).(z3.Bool)
		return child.Not()
	case TY_BOOL_AND:
		e := e.(*internalBoolExprNaryOp)
		res := s.convert(e.children[0].e, cache, symbols).(z3.Bool)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.Bool)
			res = res.And(child)
		}
		result = res
	case TY_BOOL_OR:
		e := e.(*internalBoolExprNaryOp)
		res := s.convert(e.children[0].e, cache, symbols).(z3.Bool)
		for i := 1; i < len(e.children); i++ {
			child := s.convert(e.children[i].e, cache, symbols).(z3.Bool)
			res = res.Or(child)
		}
		result = res
	default:
		panic("invalid expression type")
	}

	cache[e.rawPtr()] = result
	return result
}
