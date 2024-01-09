package gosmt

import (
	"fmt"
	"math/big"
	"runtime"
	"sort"
	"sync"
)

type bvexpr struct {
	exp     internalBVExpr
	counter int
}

type boolexpr struct {
	exp     internalBoolExpr
	counter int
}

type ExprBuilderStats struct {
	CacheHits    uint
	CacheLookups uint
	CachedBVs    uint
	CachedBools  uint
}

type ExprBuilder struct {
	lock      sync.RWMutex
	bvcache   map[uint64][]bvexpr
	boolcache map[uint64][]boolexpr

	Stats ExprBuilderStats
}

func NewExprBuilder() *ExprBuilder {
	return &ExprBuilder{
		lock:      sync.RWMutex{},
		bvcache:   map[uint64][]bvexpr{},
		boolcache: map[uint64][]boolexpr{},
		Stats:     ExprBuilderStats{},
	}
}

func (eb *ExprBuilder) PrintStats() {
	eb.lock.Lock()
	defer eb.lock.Unlock()

	fmt.Println("=====================")
	fmt.Println("  ExprBuilder Stats")
	fmt.Println("=====================")
	fmt.Printf("hits:       %d\n", eb.Stats.CacheHits)
	fmt.Printf("hit ratio:  %.03f %%\n", float64(eb.Stats.CacheHits)/float64(eb.Stats.CacheLookups)*100)
	fmt.Printf("num cached: %d\n", eb.Stats.CachedBVs+eb.Stats.CachedBools)
	fmt.Printf("bv ratio:   %.03f %%\n", float64(eb.Stats.CachedBVs)/float64(eb.Stats.CachedBVs+eb.Stats.CachedBools)*100)
	fmt.Println("=====================")
}

func (eb *ExprBuilder) bvFinalizer(e *BVExprPtr) {
	eb.lock.Lock()
	defer eb.lock.Unlock()

	h := e.e.hash()
	if _, ok := eb.bvcache[h]; !ok {
		return
	}
	buck := eb.bvcache[h]
	newBuck := make([]bvexpr, 0)
	for i := 0; i < len(buck); i++ {
		if buck[i].exp.rawPtr() == e.e.rawPtr() {
			buck[i].counter -= 1
			if buck[i].counter <= 0 {
				eb.Stats.CachedBVs -= 1
				continue
			}
		}
		newBuck = append(newBuck, buck[i])
	}
	eb.bvcache[h] = newBuck
}

func (eb *ExprBuilder) boolFinalizer(e *BoolExprPtr) {
	eb.lock.Lock()
	defer eb.lock.Unlock()

	h := e.e.hash()
	if _, ok := eb.boolcache[h]; !ok {
		return
	}
	buck := eb.boolcache[h]
	newBuck := make([]boolexpr, 0)
	for i := 0; i < len(buck); i++ {
		if buck[i].exp.rawPtr() == e.e.rawPtr() {
			buck[i].counter -= 1
			if buck[i].counter <= 0 {
				eb.Stats.CachedBools -= 1
				continue
			}
		}
		newBuck = append(newBuck, buck[i])
	}
	eb.boolcache[h] = newBuck
}

func (eb *ExprBuilder) getOrCreateBV(e internalBVExpr) *BVExprPtr {
	eb.lock.Lock()
	defer eb.lock.Unlock()
	eb.Stats.CacheLookups += 1

	h := e.hash()
	if _, ok := eb.bvcache[h]; !ok {
		eb.bvcache[h] = make([]bvexpr, 0)
	}

	bucket := eb.bvcache[h]
	for i := 0; i < len(bucket); i++ {
		if bucket[i].exp.shallowEq(e) {
			eb.Stats.CacheHits += 1

			bucket[i].counter += 1
			r := &BVExprPtr{bucket[i].exp}
			runtime.SetFinalizer(r, eb.bvFinalizer)
			return r
		}
	}
	eb.Stats.CachedBVs += 1

	bucket = append(bucket, bvexpr{e, 1})
	eb.bvcache[h] = bucket
	r := &BVExprPtr{e}
	runtime.SetFinalizer(r, eb.bvFinalizer)
	return r
}

func (eb *ExprBuilder) getOrCreateBool(e internalBoolExpr) *BoolExprPtr {
	eb.lock.Lock()
	defer eb.lock.Unlock()
	eb.Stats.CacheLookups += 1

	h := e.hash()
	if _, ok := eb.boolcache[h]; !ok {
		eb.boolcache[h] = make([]boolexpr, 0)
	}

	bucket := eb.boolcache[h]
	for i := 0; i < len(bucket); i++ {
		if bucket[i].exp.shallowEq(e) {
			eb.Stats.CacheHits += 1

			bucket[i].counter += 1
			r := &BoolExprPtr{bucket[i].exp}
			runtime.SetFinalizer(r, eb.boolFinalizer)
			return r
		}
	}
	eb.Stats.CachedBools += 1

	bucket = append(bucket, boolexpr{e, 1})
	eb.boolcache[h] = bucket
	r := &BoolExprPtr{e}
	runtime.SetFinalizer(r, eb.boolFinalizer)
	return r
}

func (eb *ExprBuilder) InvolvedInputs(e ExprPtr) []*BVExprPtr {
	queue := make([]internalExpr, 0)
	visited := make(map[uintptr]bool)
	symbols := make([]*BVExprPtr, 0)

	queue = append(queue, e.getInternal())
	for len(queue) > 0 {
		el := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if _, ok := visited[el.rawPtr()]; ok {
			continue
		}
		visited[el.rawPtr()] = true

		if el.kind() == TY_SYM {
			symel := el.(internalBVExpr)
			symbols = append(symbols, eb.getOrCreateBV(symel))
			continue
		}

		queue = append(queue, el.subexprs()...)
	}
	return symbols
}

// *** Constructors ***

func flattenOrAddArithmeticArg(e *BVExprPtr, ty int, children []*BVExprPtr) []*BVExprPtr {
	if e.Kind() == ty {
		lhsInner := e.e.(*internalBVExprBinArithmetic)
		children = append(children, lhsInner.children...)
	} else {
		children = append(children, e)
	}
	return children
}

func removeOneIf(exprs []*BVExprPtr, cmpFun func(*BVExprPtr, *BVExprPtr) bool) []*BVExprPtr {
	exprsPruned := make([]*BVExprPtr, 0)
	for i := 0; i < len(exprs); i++ {
		shouldRemove := false
		for j := i + 1; j < len(exprs); j++ {
			if cmpFun(exprs[i], exprs[j]) {
				shouldRemove = true
				break
			}
		}
		if shouldRemove {
			continue
		}
		exprsPruned = append(exprsPruned, exprs[i])
	}
	return exprsPruned
}

func removeBothIf(exprs []*BVExprPtr, cmpFun func(*BVExprPtr, *BVExprPtr) bool) []*BVExprPtr {
	removed := make(map[int]bool, 0)
	exprsPruned := make([]*BVExprPtr, 0)
	for i := 0; i < len(exprs); i++ {
		if _, ok := removed[i]; ok {
			continue
		}

		oppositeId := -1
		for j := i + 1; j < len(exprs); j++ {
			if cmpFun(exprs[i], exprs[j]) {
				oppositeId = j
				break
			}
		}
		if oppositeId >= 0 {
			removed[i] = true
			removed[oppositeId] = true
			continue
		}
		exprsPruned = append(exprsPruned, exprs[i])
	}
	return exprsPruned
}

func (eb *ExprBuilder) BVV(val int64, size uint) *BVExprPtr {
	return eb.getOrCreateBV(mkinternalBVV(val, size))
}

func (eb *ExprBuilder) BVS(name string, size uint) *BVExprPtr {
	return eb.getOrCreateBV(mkinternalBVS(name, size))
}

func (eb *ExprBuilder) Neg(e *BVExprPtr) *BVExprPtr {
	// Constant propagation
	if e.IsConst() {
		c, _ := e.GetConst()
		c.Neg()
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c))
	}

	// Neg of Neg
	if e.Kind() == TY_NEG {
		eNeg := e.e.(*internalBVExprUnArithmetic)
		return eNeg.child
	}

	// Distribute Neg over Add
	if e.Kind() == TY_ADD {
		eAdd := e.e.(*internalBVExprBinArithmetic)
		children := make([]*BVExprPtr, 0)
		for i := 0; i < len(eAdd.children); i++ {
			children = append(children, eb.Neg(eAdd.children[i]))
		}
		r, err := eb.Add(children[0], children[1])
		if err != nil {
			panic(err)
		}
		for i := 2; i < len(children); i++ {
			r, err = eb.Add(r, children[i])
			if err != nil {
				panic(err)
			}
		}
		return r
	}

	ex, _ := mkinternalBVExprNeg(e)
	return eb.getOrCreateBV(ex)
}

func (eb *ExprBuilder) Not(e *BVExprPtr) *BVExprPtr {
	// Constant propagation
	if e.IsConst() {
		c, _ := e.GetConst()
		c.Not()
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c))
	}

	// Not of Not
	if e.Kind() == TY_NOT {
		eNot := e.e.(*internalBVExprUnArithmetic)
		return eNot.child
	}

	ex, _ := mkinternalBVExprNot(e)
	return eb.getOrCreateBV(ex)
}

func (eb *ExprBuilder) Add(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if err := c2.Add(c1); err != nil {
			return nil, err
		}
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Remove zeroes
	if lhs.IsZero() {
		return rhs, nil
	}
	if rhs.IsZero() {
		return lhs, nil
	}

	// Remove add with opposite
	if lhs.IsOppositeOf(rhs) {
		return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
	}

	childrenFlattened := make([]*BVExprPtr, 0)
	childrenFlattened = flattenOrAddArithmeticArg(lhs, TY_ADD, childrenFlattened)
	childrenFlattened = flattenOrAddArithmeticArg(rhs, TY_ADD, childrenFlattened)

	// Constant propagation
	children := make([]*BVExprPtr, 0)
	cVal := MakeBVConst(0, lhs.Size())
	for i := 0; i < len(childrenFlattened); i++ {
		child := childrenFlattened[i]
		if child.IsConst() {
			childConst, _ := child.GetConst()
			cVal.Add(childConst)
		} else {
			children = append(children, child)
		}
	}
	if !cVal.IsZero() {
		children = append(children, eb.getOrCreateBV(mkinternalBVVFromConst(*cVal)))
	}
	if len(children) == 0 {
		return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
	}
	if len(children) == 1 {
		return children[0], nil
	}

	// Remove add with opposite on flattened
	if len(children) > 2 {
		children = removeBothIf(children, func(bp1, bp2 *BVExprPtr) bool { return bp1.IsOppositeOf(bp2) })
		if len(children) == 0 {
			return eb.BVV(0, lhs.Size()), nil
		}
		if len(children) == 1 {
			return children[0], nil
		}
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBVExprAdd(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Mul(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Remove ones
	if lhs.IsOne() {
		return rhs, nil
	}
	if rhs.IsOne() {
		return lhs, nil
	}

	// Check zero
	if lhs.IsZero() {
		return lhs, nil
	}
	if rhs.IsZero() {
		return rhs, nil
	}

	childrenFlattened := make([]*BVExprPtr, 0)
	childrenFlattened = flattenOrAddArithmeticArg(lhs, TY_MUL, childrenFlattened)
	childrenFlattened = flattenOrAddArithmeticArg(rhs, TY_MUL, childrenFlattened)

	// Constant propagation
	children := make([]*BVExprPtr, 0)
	cVal := MakeBVConst(1, lhs.Size())
	for i := 0; i < len(childrenFlattened); i++ {
		child := childrenFlattened[i]
		if child.IsConst() {
			childConst, _ := child.GetConst()
			cVal.Mul(childConst)
		} else {
			children = append(children, child)
		}
	}
	if !cVal.IsOne() {
		children = append(children, eb.getOrCreateBV(mkinternalBVVFromConst(*cVal)))
	}
	if len(children) == 0 {
		return eb.getOrCreateBV(mkinternalBVV(1, lhs.Size())), nil
	}
	if len(children) == 1 {
		return children[0], nil
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBVExprMul(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) And(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Check zero
	if lhs.IsZero() {
		return lhs, nil
	}
	if rhs.IsZero() {
		return rhs, nil
	}

	// Check if all bit set
	if lhs.HasAllBitsSet() {
		return rhs, nil
	}
	if rhs.HasAllBitsSet() {
		return lhs, nil
	}

	// Check if lhs == rhs
	if lhs.Id() == rhs.Id() {
		return lhs, nil
	}

	childrenFlattened := make([]*BVExprPtr, 0)
	childrenFlattened = flattenOrAddArithmeticArg(lhs, TY_AND, childrenFlattened)
	childrenFlattened = flattenOrAddArithmeticArg(rhs, TY_AND, childrenFlattened)

	// Constant propagation
	children := make([]*BVExprPtr, 0)
	cVal := MakeBVConst(-1, lhs.Size())
	for i := 0; i < len(childrenFlattened); i++ {
		child := childrenFlattened[i]
		if child.IsConst() {
			childConst, _ := child.GetConst()
			cVal.And(childConst)
		} else {
			children = append(children, child)
		}
	}
	if !cVal.HasAllBitsSet() {
		children = append(children, eb.getOrCreateBV(mkinternalBVVFromConst(*cVal)))
	}
	if len(children) == 0 {
		return eb.getOrCreateBV(mkinternalBVV(-1, lhs.Size())), nil
	}
	if len(children) == 1 {
		return children[0], nil
	}

	// Remove and with same on flattened
	if len(children) > 2 {
		children = removeOneIf(children, func(bp1, bp2 *BVExprPtr) bool { return bp1.Id() == bp2.Id() })
		if len(children) == 0 {
			panic("should not happen")
		}
		if len(children) == 1 {
			return children[0], nil
		}
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBVExprAnd(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Or(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Check zero
	if lhs.IsZero() {
		return rhs, nil
	}
	if rhs.IsZero() {
		return lhs, nil
	}

	// Check if all bit set
	if lhs.HasAllBitsSet() {
		return lhs, nil
	}
	if rhs.HasAllBitsSet() {
		return rhs, nil
	}

	// Check if lhs == rhs
	if lhs.Id() == rhs.Id() {
		return lhs, nil
	}

	childrenFlattened := make([]*BVExprPtr, 0)
	childrenFlattened = flattenOrAddArithmeticArg(lhs, TY_OR, childrenFlattened)
	childrenFlattened = flattenOrAddArithmeticArg(rhs, TY_OR, childrenFlattened)

	// Constant propagation
	children := make([]*BVExprPtr, 0)
	cVal := MakeBVConst(0, lhs.Size())
	for i := 0; i < len(childrenFlattened); i++ {
		child := childrenFlattened[i]
		if child.IsConst() {
			childConst, _ := child.GetConst()
			cVal.Or(childConst)
		} else {
			children = append(children, child)
		}
	}
	if !cVal.IsZero() {
		children = append(children, eb.getOrCreateBV(mkinternalBVVFromConst(*cVal)))
	}
	if len(children) == 0 {
		return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
	}
	if len(children) == 1 {
		return children[0], nil
	}

	// Remove and with same on flattened
	if len(children) > 2 {
		children = removeOneIf(children, func(bp1, bp2 *BVExprPtr) bool { return bp1.Id() == bp2.Id() })
		if len(children) == 0 {
			panic("should not happen")
		}
		if len(children) == 1 {
			return children[0], nil
		}
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBVExprOr(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Xor(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Check zero
	if lhs.IsZero() {
		return rhs, nil
	}
	if rhs.IsZero() {
		return lhs, nil
	}

	// Check if same
	if lhs.Id() == rhs.Id() {
		return eb.BVV(0, lhs.Size()), nil
	}

	childrenFlattened := make([]*BVExprPtr, 0)
	childrenFlattened = flattenOrAddArithmeticArg(lhs, TY_XOR, childrenFlattened)
	childrenFlattened = flattenOrAddArithmeticArg(rhs, TY_XOR, childrenFlattened)

	// Constant propagation
	children := make([]*BVExprPtr, 0)
	cVal := MakeBVConst(0, lhs.Size())
	for i := 0; i < len(childrenFlattened); i++ {
		child := childrenFlattened[i]
		if child.IsConst() {
			childConst, _ := child.GetConst()
			cVal.Xor(childConst)
		} else {
			children = append(children, child)
		}
	}
	if !cVal.IsZero() {
		children = append(children, eb.getOrCreateBV(mkinternalBVVFromConst(*cVal)))
	}
	if len(children) == 0 {
		return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
	}
	if len(children) == 1 {
		return children[0], nil
	}

	// Remove couples of same expression on flattened
	if len(children) > 2 {
		children = removeBothIf(children, func(bp1, bp2 *BVExprPtr) bool { return bp1.Id() == bp2.Id() })
		if len(children) == 0 {
			return eb.BVV(0, lhs.Size()), nil
		}
		if len(children) == 1 {
			return children[0], nil
		}
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBVExprXor(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Shl(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if !c2.FitInLong() {
			return eb.getOrCreateBV(mkinternalBVV(0, c1.Size)), nil
		}
		c1.Shl(uint(c2.AsULong()))
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Check if shift with rhs > lhs.Size or rhs == 0
	if rhs.IsConst() {
		n, _ := rhs.GetConst()
		if n.value.Cmp(zero) == 0 {
			return lhs, nil
		}
		if n.value.Cmp(big.NewInt(int64(lhs.Size()))) >= 0 {
			return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
		}
	}

	ex, err := mkinternalBVExprShl(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) LShr(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if !c2.FitInLong() {
			return eb.getOrCreateBV(mkinternalBVV(0, c1.Size)), nil
		}
		c1.LShr(uint(c2.AsULong()))
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Check if shift with rhs > lhs.Size or rhs == 0
	if rhs.IsConst() {
		n, _ := rhs.GetConst()
		if n.value.Cmp(zero) == 0 {
			return lhs, nil
		}
		if n.value.Cmp(big.NewInt(int64(lhs.Size()))) >= 0 {
			return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
		}
	}

	ex, err := mkinternalBVExprLshr(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) AShr(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if !c2.FitInLong() {
			return eb.getOrCreateBV(mkinternalBVV(0, c1.Size)), nil
		}
		c1.LShr(uint(c2.AsULong()))
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Check if shift with rhs > lhs.Size or rhs == 0
	if rhs.IsConst() {
		n, _ := rhs.GetConst()
		if n.value.Cmp(zero) == 0 {
			return lhs, nil
		}
		if n.value.Cmp(big.NewInt(int64(lhs.Size()))) >= 0 {
			return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
		}
	}

	ex, err := mkinternalBVExprAshr(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Extract(e *BVExprPtr, high, low uint) (*BVExprPtr, error) {
	if high < low {
		return nil, fmt.Errorf("high < low")
	}
	if e.Size() < high-low+1 {
		return nil, fmt.Errorf("high-low+1 > e.Size")
	}

	// Reduntant extract
	if low == 0 && high == e.Size()-1 {
		return e, nil
	}

	// Constant propagation
	if e.IsConst() {
		c, _ := e.GetConst()
		err := c.Truncate(high, low)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c)), nil
	}

	// Extract of extract
	if e.Kind() == TY_EXTRACT {
		eInt := e.e.(*internalBVExprExtract)
		newLow := low + eInt.low
		newHigh := high + eInt.high
		ex, err := mkinternalBVExprExtract(eInt.child, newHigh, newLow)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBV(ex), nil
	}

	// Extract of concat
	if e.Kind() == TY_CONCAT {
		eInt := e.e.(*internalBVExprConcat)
		off := e.Size()
		for i := 0; i < len(eInt.children); i++ {
			child := eInt.children[i]
			off -= child.Size()
			concatHigh := child.Size() + off - 1
			concatLow := off
			if concatHigh >= high && low >= concatLow {
				return eb.Extract(child, high-off, low-off)
			}
		}
	}

	// Extract of ZEXT
	if e.Kind() == TY_ZEXT {
		eInt := e.e.(*internalBVExprExtend)
		if low == 0 && high == eInt.child.Size()-1 {
			return eInt.child, nil
		}
		if low >= eInt.child.Size() {
			return eb.BVV(0, high-low+1), nil
		}
		ex, err := eb.Extract(eInt.child, min(high, eInt.child.Size()-1), low)
		if err != nil {
			return nil, err
		}
		return eb.ZExt(ex, high-low+1-ex.Size())
	}

	// Extract of SEXT
	if e.Kind() == TY_SEXT {
		eInt := e.e.(*internalBVExprExtend)
		if low == 0 && high == eInt.child.Size()-1 {
			return eInt.child, nil
		}
		if high < eInt.child.Size() {
			return eb.Extract(eInt.child, high, low)
		}
	}

	ex, err := mkinternalBVExprExtract(e, high, low)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) ZExt(e *BVExprPtr, n uint) (*BVExprPtr, error) {
	// Unnecessary ZExt
	if n == 0 {
		return e, nil
	}

	// ZExt of ZExt
	if e.Kind() == TY_ZEXT {
		eInt := e.e.(*internalBVExprExtend)
		return eb.ZExt(eInt.child, eInt.n+n)
	}

	// Constant propagation
	if e.IsConst() {
		c, _ := e.GetConst()
		c.ZExt(n)
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c)), nil
	}

	ex, err := mkinternalBVExprZExt(e, n)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) SExt(e *BVExprPtr, n uint) (*BVExprPtr, error) {
	// Unnecessary SExt
	if n == 0 {
		return e, nil
	}

	// SExt of SExt
	if e.Kind() == TY_SEXT {
		eInt := e.e.(*internalBVExprExtend)
		return eb.SExt(eInt.child, eInt.n+n)
	}

	// SExt of ZExt
	if e.Kind() == TY_ZEXT {
		eInt := e.e.(*internalBVExprExtend)
		if eInt.n == 0 {
			panic("zext with n==0")
		}
		return eb.ZExt(eInt.child, eInt.n+n)
	}

	// Constant propagation
	if e.IsConst() {
		c, _ := e.GetConst()
		c.SExt(n)
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c)), nil
	}

	ex, err := mkinternalBVExprSExt(e, n)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Concat(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	// Pattern SExt(EXPR)[high:EXPR.size] # EXPR ==> sext(EXPR)
	if lhs.Kind() == TY_EXTRACT {
		lhsInt := lhs.e.(*internalBVExprExtract)
		if lhsInt.low == rhs.Size() && lhsInt.child.Kind() == TY_SEXT {
			lhsChildInt := lhsInt.child.e.(*internalBVExprExtend)
			if lhsChildInt.child.Id() == rhs.Id() {
				return eb.SExt(rhs, lhs.Size())
			}
		}
	}

	// Pattern sext(EXPR)[high:N] # sext(EXPR, N) ==> sext(EXPR)
	if lhs.Kind() == TY_EXTRACT && rhs.Kind() == TY_SEXT {
		lhsInt := lhs.e.(*internalBVExprExtract)
		rhsInt := rhs.e.(*internalBVExprExtend)
		if lhsInt.low == rhs.Size() && lhsInt.child.Kind() == TY_SEXT {
			lhsChildInt := lhsInt.child.e.(*internalBVExprExtend)
			if lhsChildInt.child.Id() == rhsInt.child.Id() {
				return eb.SExt(rhsInt.child, lhs.Size())
			}
		}
	}

	// Flatten arguments
	children := make([]*BVExprPtr, 0)
	if lhs.Kind() == TY_CONCAT {
		lhsInner := lhs.e.(*internalBVExprConcat)
		children = append(children, lhsInner.children...)
	} else {
		children = append(children, lhs)
	}
	if rhs.Kind() == TY_CONCAT {
		rhsInner := rhs.e.(*internalBVExprConcat)
		children = append(children, rhsInner.children...)
	} else {
		children = append(children, rhs)
	}

	// Constant propagation
	constpropChildren := make([]*BVExprPtr, 0)
	for i := 0; i < len(children); i += 1 {
		child := children[i]
		if child.IsConst() {
			conc, _ := child.GetConst()

			var j int
			for j = i + 1; j < len(children); j++ {
				nextChild := children[j]
				if !nextChild.IsConst() {
					break
				}
				nextConc, _ := nextChild.GetConst()
				conc.Concat(nextConc)
			}
			i = j - 1
			constpropChildren = append(
				constpropChildren, eb.getOrCreateBV(mkinternalBVVFromConst(*conc)))
		} else {
			constpropChildren = append(constpropChildren, child)
		}
	}

	// Concat of Extract
	mergedExtractChildren := make([]*BVExprPtr, 0)
	for i := 0; i < len(constpropChildren); i += 1 {
		child := constpropChildren[i]
		if child.Kind() == TY_EXTRACT {
			childInt := child.e.(*internalBVExprExtract)

			high := childInt.high
			low := childInt.low

			var j int
			for j = i + 1; j < len(constpropChildren); j++ {
				nextChild := children[j]
				if nextChild.Kind() != TY_EXTRACT {
					break
				}
				nextChildInt := nextChild.e.(*internalBVExprExtract)
				if nextChildInt.child.Id() != childInt.child.Id() {
					break
				}
				if low != nextChildInt.high+1 {
					break
				}
				low = nextChildInt.low
			}
			i = j - 1
			ex, err := eb.Extract(childInt.child, high, low)
			if err != nil {
				return nil, err
			}
			mergedExtractChildren = append(mergedExtractChildren, ex)
		} else {
			mergedExtractChildren = append(mergedExtractChildren, child)
		}
	}

	if len(mergedExtractChildren) == 0 {
		panic("concat has no children")
	}
	if len(mergedExtractChildren) == 1 {
		return mergedExtractChildren[0], nil
	}

	ex, err := mkinternalBVExprConcat(mergedExtractChildren)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) UDiv(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if c2.IsZero() {
			// We are consistent with Z3 (div by zero yelds -1)
			return eb.getOrCreateBV(mkinternalBVV(-1, c1.Size)), nil
		}
		c1.UDiv(c2)
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Div by myself
	if lhs.Id() == rhs.Id() {
		return eb.getOrCreateBV(mkinternalBVV(1, lhs.Size())), nil
	}

	ex, err := mkinternalBVExprUdiv(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) SDiv(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if c2.IsZero() {
			// We are consistent with Z3 (div by zero yelds -1)
			return eb.getOrCreateBV(mkinternalBVV(-1, c1.Size)), nil
		}
		c1.SDiv(c2)
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Div by myself
	if lhs.Id() == rhs.Id() {
		return eb.getOrCreateBV(mkinternalBVV(1, lhs.Size())), nil
	}

	ex, err := mkinternalBVExprSdiv(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) URem(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if c2.IsZero() {
			// We are consistent with Z3 (rem by zero yields lhs)
			return lhs, nil
		}
		c1.URem(c2)
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Rem by myself
	if lhs.Id() == rhs.Id() {
		return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
	}
	// Rem by one
	if rhs.IsConst() {
		c, _ := rhs.GetConst()
		if c.IsOne() {
			return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
		}
	}

	ex, err := mkinternalBVExprUrem(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) SRem(lhs, rhs *BVExprPtr) (*BVExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		if c2.IsZero() {
			// We are consistent with Z3 (rem by zero yields lhs)
			return lhs, nil
		}
		c1.SRem(c2)
		return eb.getOrCreateBV(mkinternalBVVFromConst(*c1)), nil
	}

	// Rem by myself
	if lhs.Id() == rhs.Id() {
		return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
	}
	// Rem by one
	if rhs.IsConst() {
		c, _ := rhs.GetConst()
		if c.IsOne() {
			return eb.getOrCreateBV(mkinternalBVV(0, lhs.Size())), nil
		}
	}

	ex, err := mkinternalBVExprSrem(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) ITE(guard *BoolExprPtr, iftrue *BVExprPtr, iffalse *BVExprPtr) (*BVExprPtr, error) {
	if iftrue.Size() != iffalse.Size() {
		return nil, fmt.Errorf("invalid sizes in ITE")
	}

	// Constant propagation
	if guard.IsConst() {
		g, _ := guard.GetConst()
		if g {
			return iftrue, nil
		}
		return iffalse, nil
	}

	ex, err := mkinternalBVExprITE(guard, iftrue, iffalse)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBV(ex), nil
}

func (eb *ExprBuilder) Ult(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.Ult(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprUlt(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) Ule(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.Ule(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprUle(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) UGt(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.UGt(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprUgt(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) UGe(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.UGe(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprUge(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) SLt(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.SLt(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprSlt(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) SLe(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.SLe(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprSle(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) SGt(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.SGt(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprSgt(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) SGe(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.SGe(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprSge(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) Eq(lhs, rhs *BVExprPtr) (*BoolExprPtr, error) {
	if lhs.Size() != rhs.Size() {
		return nil, fmt.Errorf("different sizes")
	}

	// Constant propagation
	if lhs.IsConst() && rhs.IsConst() {
		c1, _ := lhs.GetConst()
		c2, _ := rhs.GetConst()
		r, err := c1.Eq(c2)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(mkinternalBoolConst(r.Value)), nil
	}

	ex, err := mkinternalBoolExprEq(lhs, rhs)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) BoolVal(v bool) *BoolExprPtr {
	return eb.getOrCreateBool(mkinternalBoolConst(v))
}

func (eb *ExprBuilder) BoolNot(e *BoolExprPtr) (*BoolExprPtr, error) {
	// Constant propagation
	if e.IsConst() {
		v, _ := e.GetConst()
		return eb.getOrCreateBool(mkinternalBoolConst(!v)), nil
	}

	// Not of Not
	if e.Kind() == TY_BOOL_NOT {
		eBoolNot := e.e.(*internalBoolUnArithmetic)
		return eBoolNot.child, nil
	}

	// Distribute Not over And (De Morgan)
	if e.Kind() == TY_BOOL_AND {
		eInt := e.e.(*internalBoolExprNaryOp)
		children := make([]*BoolExprPtr, 0)
		for i := 0; i < len(eInt.children); i++ {
			child, err := eb.BoolNot(eInt.children[i])
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		r, err := eb.BoolOr(children[0], children[1])
		if err != nil {
			return nil, err
		}
		for i := 2; i < len(children); i++ {
			r, err = eb.BoolOr(r, children[i])
			if err != nil {
				return nil, err
			}
		}
		return r, nil
	}

	// Distribute Not over Or (De Morgan)
	if e.Kind() == TY_BOOL_OR {
		eInt := e.e.(*internalBoolExprNaryOp)
		children := make([]*BoolExprPtr, 0)
		for i := 0; i < len(eInt.children); i++ {
			child, err := eb.BoolNot(eInt.children[i])
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		r, err := eb.BoolAnd(children[0], children[1])
		if err != nil {
			return nil, err
		}
		for i := 2; i < len(children); i++ {
			r, err = eb.BoolAnd(r, children[i])
			if err != nil {
				return nil, err
			}
		}
		return r, nil
	}

	// Not of { Ule, Ult, Uge, Ugt, Sle, Slt, Sge, Sgt }
	if e.Kind() == TY_ULE {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprUgt(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_ULT {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprUge(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_UGE {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprUlt(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_UGT {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprUle(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_SLE {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprSgt(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_SLT {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprSge(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_SGT {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprSle(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}
	if e.Kind() == TY_SGE {
		eInt := e.e.(*internalBoolExprCmp)
		ex, err := mkinternalBoolExprSlt(eInt.lhs, eInt.rhs)
		if err != nil {
			return nil, err
		}
		return eb.getOrCreateBool(ex), nil
	}

	ex, err := mkinternalBoolNot(e)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) BoolAnd(lhs, rhs *BoolExprPtr) (*BoolExprPtr, error) {
	// Constant propagation
	if lhs.IsConst() {
		lhsV, _ := lhs.GetConst()
		if lhsV {
			return rhs, nil
		}
		return eb.getOrCreateBool(mkinternalBoolConst(false)), nil
	}
	if rhs.IsConst() {
		rhsV, _ := rhs.GetConst()
		if rhsV {
			return lhs, nil
		}
		return eb.getOrCreateBool(mkinternalBoolConst(false)), nil
	}

	// Flatten args
	children := make([]*BoolExprPtr, 0)
	if lhs.Kind() == TY_BOOL_AND {
		lhsInner := lhs.e.(*internalBoolExprNaryOp)
		children = append(children, lhsInner.children...)
	} else {
		children = append(children, lhs)
	}
	if rhs.Kind() == TY_BOOL_AND {
		rhsInner := rhs.e.(*internalBoolExprNaryOp)
		children = append(children, rhsInner.children...)
	} else {
		children = append(children, rhs)
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBoolExprAnd(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}

func (eb *ExprBuilder) BoolOr(lhs, rhs *BoolExprPtr) (*BoolExprPtr, error) {
	// Constant propagation
	if lhs.IsConst() {
		lhsV, _ := lhs.GetConst()
		if !lhsV {
			return rhs, nil
		}
		return eb.getOrCreateBool(mkinternalBoolConst(true)), nil
	}
	if rhs.IsConst() {
		rhsV, _ := rhs.GetConst()
		if !rhsV {
			return lhs, nil
		}
		return eb.getOrCreateBool(mkinternalBoolConst(true)), nil
	}

	// Flatten args
	children := make([]*BoolExprPtr, 0)
	if lhs.Kind() == TY_BOOL_OR {
		lhsInner := lhs.e.(*internalBoolExprNaryOp)
		children = append(children, lhsInner.children...)
	} else {
		children = append(children, lhs)
	}
	if rhs.Kind() == TY_BOOL_OR {
		rhsInner := rhs.e.(*internalBoolExprNaryOp)
		children = append(children, rhsInner.children...)
	} else {
		children = append(children, rhs)
	}

	sort.Slice(children[:], func(i, j int) bool { return children[i].Id() < children[j].Id() })
	ex, err := mkinternalBoolExprOr(children)
	if err != nil {
		return nil, err
	}
	return eb.getOrCreateBool(ex), nil
}
