package gosmt

func (eb *ExprBuilder) eval(e ExprPtr, interpr map[string]*BVConst) ExprPtr {
	cache := make(map[uintptr]ExprPtr)
	return eb.eval_internal(e, cache, interpr)
}

func (eb *ExprBuilder) eval_internal(eptr ExprPtr, cache map[uintptr]ExprPtr, interpr map[string]*BVConst) ExprPtr {
	e := eptr.getInternal()
	if r, ok := cache[e.rawPtr()]; ok {
		return r
	}

	var result ExprPtr
	var err error = nil
	switch e.kind() {
	case TY_SYM:
		bv := e.(*internalBVS)
		if c, ok := interpr[bv.name]; ok {
			cInt := mkinternalBVVFromConst(*c)
			return eb.getOrCreateBV(cInt)
		}
		return eptr
	case TY_CONST:
		return eptr
	case TY_EXTRACT:
		e := e.(*internalBVExprExtract)
		child := eb.eval_internal(e.child, cache, interpr).(*BVExprPtr)
		result, err = eb.Extract(child, e.high, e.low)
	case TY_CONCAT:
		e := e.(*internalBVExprConcat)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BVExprPtr)
			res, err = eb.Concat(res, child)
		}
		result = res
	case TY_ZEXT:
		e := e.(*internalBVExprExtend)
		child := eb.eval_internal(e.child, cache, interpr).(*BVExprPtr)
		result, err = eb.ZExt(child, e.n)
	case TY_SEXT:
		e := e.(*internalBVExprExtend)
		child := eb.eval_internal(e.child, cache, interpr).(*BVExprPtr)
		result, err = eb.SExt(child, e.n)
	case TY_ITE:
		e := e.(*internalBVExprITE)
		guard := eb.eval_internal(e.cond, cache, interpr).(*BoolExprPtr)
		iftrue := eb.eval_internal(e.iftrue, cache, interpr).(*BVExprPtr)
		iffalse := eb.eval_internal(e.iffalse, cache, interpr).(*BVExprPtr)
		result, err = eb.ITE(guard, iftrue, iffalse)
	case TY_NOT:
		e := e.(*internalBVExprUnArithmetic)
		child := eb.eval_internal(e.child, cache, interpr).(*BVExprPtr)
		result = eb.Not(child)
	case TY_NEG:
		e := e.(*internalBVExprUnArithmetic)
		child := eb.eval_internal(e.child, cache, interpr).(*BVExprPtr)
		result = eb.Neg(child)
	case TY_SHL:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.Shl(lhs, rhs)
	case TY_LSHR:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.LShr(lhs, rhs)
	case TY_ASHR:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.AShr(lhs, rhs)
	case TY_AND:
		e := e.(*internalBVExprBinArithmetic)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BVExprPtr)
			res, err = eb.And(res, child)
			if err != nil {
				break
			}
		}
		result = res
	case TY_OR:
		e := e.(*internalBVExprBinArithmetic)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BVExprPtr)
			res, err = eb.Or(res, child)
			if err != nil {
				break
			}
		}
		result = res
	case TY_XOR:
		e := e.(*internalBVExprBinArithmetic)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BVExprPtr)
			res, err = eb.Xor(res, child)
			if err != nil {
				break
			}
		}
		result = res
	case TY_ADD:
		e := e.(*internalBVExprBinArithmetic)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BVExprPtr)
			res, err = eb.Add(res, child)
			if err != nil {
				break
			}
		}
		result = res
	case TY_MUL:
		e := e.(*internalBVExprBinArithmetic)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BVExprPtr)
			res, err = eb.Mul(res, child)
			if err != nil {
				break
			}
		}
		result = res
	case TY_SDIV:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.SDiv(lhs, rhs)
	case TY_UDIV:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.UDiv(lhs, rhs)
	case TY_SREM:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.SRem(lhs, rhs)
	case TY_UREM:
		e := e.(*internalBVExprBinArithmetic)
		lhs := eb.eval_internal(e.children[0], cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.children[1], cache, interpr).(*BVExprPtr)
		result, err = eb.URem(lhs, rhs)
	case TY_ULT:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.Ult(lhs, rhs)
	case TY_ULE:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.Ule(lhs, rhs)
	case TY_UGT:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.UGt(lhs, rhs)
	case TY_UGE:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.UGe(lhs, rhs)
	case TY_SLT:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.SLt(lhs, rhs)
	case TY_SLE:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.SLe(lhs, rhs)
	case TY_SGT:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.SGt(lhs, rhs)
	case TY_SGE:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.SGe(lhs, rhs)
	case TY_EQ:
		e := e.(*internalBoolExprCmp)
		lhs := eb.eval_internal(e.lhs, cache, interpr).(*BVExprPtr)
		rhs := eb.eval_internal(e.rhs, cache, interpr).(*BVExprPtr)
		result, err = eb.Eq(lhs, rhs)
	case TY_BOOL_CONST:
		e := e.(*internalBoolVal)
		result = eb.BoolVal(e.Value.Value)
	case TY_BOOL_NOT:
		e := e.(*internalBoolUnArithmetic)
		child := eb.eval_internal(e.child, cache, interpr).(*BoolExprPtr)
		result, err = eb.BoolNot(child)
	case TY_BOOL_AND:
		e := e.(*internalBoolExprNaryOp)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BoolExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BoolExprPtr)
			res, err = eb.BoolAnd(res, child)
			if err != nil {
				break
			}
		}
		result = res
	case TY_BOOL_OR:
		e := e.(*internalBoolExprNaryOp)
		res := eb.eval_internal(e.children[0], cache, interpr).(*BoolExprPtr)
		for i := 1; i < len(e.children); i++ {
			child := eb.eval_internal(e.children[i], cache, interpr).(*BoolExprPtr)
			res, err = eb.BoolOr(res, child)
		}
		result = res
	default:
		panic("invalid expression type")
	}

	if err != nil {
		panic(err)
	}

	cache[e.rawPtr()] = result
	return result
}
