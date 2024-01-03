package gosmt

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unsafe"

	"github.com/cespare/xxhash/v2"
)

const (
	TY_SYM     = 1
	TY_CONST   = 2
	TY_EXTRACT = 3
	TY_CONCAT  = 4
	TY_ZEXT    = 5
	TY_SEXT    = 6
	TY_ITE     = 7

	TY_NOT  = 8
	TY_NEG  = 9
	TY_SHL  = 10
	TY_LSHR = 11
	TY_ASHR = 12
	TY_AND  = 13
	TY_OR   = 14
	TY_XOR  = 15
	TY_ADD  = 16
	TY_MUL  = 17
	TY_SDIV = 18
	TY_UDIV = 19
	TY_SREM = 20
	TY_UREM = 21

	TY_ULT = 22
	TY_ULE = 23
	TY_UGT = 24
	TY_UGE = 25
	TY_SLT = 26
	TY_SLE = 27
	TY_SGT = 28
	TY_SGE = 29
	TY_EQ  = 30

	TY_BOOL_CONST = 31
	TY_BOOL_NOT   = 32
	TY_BOOL_AND   = 33
	TY_BOOL_OR    = 34
)

/*
 *   Public Interface
 */

type BVExprPtr struct {
	e internalBVExpr
}

func (bv *BVExprPtr) IsConst() bool {
	return bv.e.kind() == TY_CONST
}

func (bv *BVExprPtr) GetConst() (*BVConst, error) {
	if bv.e.kind() != TY_CONST {
		return nil, fmt.Errorf("not a constant")
	}
	c := bv.e.(*internalBVV)
	return c.Value.Copy(), nil
}

func (bv *BVExprPtr) IsZero() bool {
	if !bv.IsConst() {
		return false
	}
	c, _ := bv.GetConst()
	return c.IsZero()
}

func (bv *BVExprPtr) IsOne() bool {
	if !bv.IsConst() {
		return false
	}
	c, _ := bv.GetConst()
	return c.IsOne()
}

func (bv *BVExprPtr) HasAllBitsSet() bool {
	if !bv.IsConst() {
		return false
	}
	c, _ := bv.GetConst()
	return c.HasAllBitsSet()
}

func (bv *BVExprPtr) IsOppositeOf(o *BVExprPtr) bool {
	if bv.Kind() == TY_NEG {
		negBv := bv.e.(*internalBVExprUnArithmetic)
		if o.Id() == negBv.child.Id() {
			return true
		}
	}
	if o.Kind() == TY_NEG {
		negO := o.e.(*internalBVExprUnArithmetic)
		return bv.Id() == negO.child.Id()
	}
	return false
}

func (bv *BVExprPtr) Size() uint {
	return bv.e.size()
}

func (bv *BVExprPtr) String() string {
	return bv.e.String()
}

func (bv *BVExprPtr) Id() uintptr {
	return bv.e.rawPtr()
}

func (bv *BVExprPtr) Kind() int {
	return bv.e.kind()
}

type BoolExprPtr struct {
	e internalBoolExpr
}

func (e *BoolExprPtr) IsConst() bool {
	return e.e.kind() == TY_BOOL_CONST
}

func (e *BoolExprPtr) GetConst() (bool, error) {
	if e.e.kind() != TY_BOOL_CONST {
		return false, fmt.Errorf("not a constant")
	}
	c := e.e.(*internalBoolVal)
	return c.Value.Value, nil
}

func (e *BoolExprPtr) String() string {
	return e.e.String()
}

func (e *BoolExprPtr) Id() uintptr {
	return e.e.rawPtr()
}

func (e *BoolExprPtr) Kind() int {
	return e.e.kind()
}

/*
 *   Private Interface
 */

type internalExpr interface {
	String() string

	kind() int
	hash() uint64
	isLeaf() bool
	rawPtr() uintptr
	subexprs() []internalExpr
}

type internalBVExpr interface {
	internalExpr

	size() uint
	deepEq(internalBVExpr) bool
	shallowEq(internalBVExpr) bool
}

type internalBoolExpr interface {
	internalExpr

	deepEq(internalBoolExpr) bool
	shallowEq(internalBoolExpr) bool
}

/*
 *  TY_CONST
 */

type internalBVV struct {
	Value BVConst
}

func mkinternalBVV(value int64, size uint) *internalBVV {
	return &internalBVV{Value: *MakeBVConst(value, size)}
}

func mkinternalBVVFromConst(c BVConst) *internalBVV {
	return &internalBVV{Value: c}
}

func (bvv *internalBVV) String() string {
	return fmt.Sprintf("0x%x", bvv.Value.value)
}

func (bvv *internalBVV) size() uint {
	return bvv.Value.Size
}

func (bvv *internalBVV) subexprs() []internalExpr {
	return make([]internalExpr, 0)
}

func (bvv *internalBVV) kind() int {
	return TY_CONST
}

func (bvv *internalBVV) hash() uint64 {
	if bvv.Value.Size > 64 {
		cpy := bvv.Value.Copy()
		cpy.Truncate(63, 0)
		return cpy.AsULong()
	}
	return bvv.Value.AsULong()
}

func (bvv *internalBVV) deepEq(other internalBVExpr) bool {
	if other.kind() != TY_CONST {
		return false
	}
	obvv := other.(*internalBVV)
	res, err := bvv.Value.Eq(&obvv.Value)
	if err != nil || !res.Value {
		return false
	}
	return true
}

func (bvv *internalBVV) shallowEq(other internalBVExpr) bool {
	return bvv.deepEq(other)
}

func (bvv *internalBVV) isLeaf() bool {
	return true
}

func (bvv *internalBVV) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(bvv))
}

/*
 *  TY_BOOL_CONST
 */

type internalBoolVal struct {
	Value BoolConst
}

func mkinternalBoolConst(value bool) *internalBoolVal {
	if value {
		return &internalBoolVal{Value: BoolTrue()}
	}
	return &internalBoolVal{Value: BoolFalse()}
}

func (b *internalBoolVal) String() string {
	return b.Value.String()
}

func (b *internalBoolVal) subexprs() []internalExpr {
	return make([]internalExpr, 0)
}

func (b *internalBoolVal) kind() int {
	return TY_BOOL_CONST
}

func (b *internalBoolVal) hash() uint64 {
	if b.Value.Value {
		return 1
	}
	return 0
}

func (b *internalBoolVal) deepEq(other internalBoolExpr) bool {
	if other.kind() != TY_BOOL_CONST {
		return false
	}
	ob := other.(*internalBoolVal)
	return ob.Value.Value == b.Value.Value
}

func (b *internalBoolVal) shallowEq(other internalBoolExpr) bool {
	return b.deepEq(other)
}

func (b *internalBoolVal) isLeaf() bool {
	return true
}

func (b *internalBoolVal) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(b))
}

/*
 *  TY_SYM
 */

type internalBVS struct {
	name string
	sz   uint
}

func mkinternalBVS(name string, size uint) *internalBVS {
	return &internalBVS{name: name, sz: size}
}

func (bvs *internalBVS) String() string {
	return bvs.name
}

func (bvs *internalBVS) size() uint {
	return bvs.sz
}

func (bvs *internalBVS) subexprs() []internalExpr {
	return make([]internalExpr, 0)
}

func (bvs *internalBVS) kind() int {
	return TY_SYM
}

func (bvs *internalBVS) hash() uint64 {
	h := xxhash.New()
	n, err := h.Write([]byte(bvs.name))
	if err != nil || n != len(bvs.name) {
		panic(err)
	}
	return h.Sum64()
}

func (bvs *internalBVS) deepEq(other internalBVExpr) bool {
	if other.kind() != TY_SYM {
		return false
	}
	obvs := other.(*internalBVS)
	return obvs.sz == bvs.sz && obvs.name == bvs.name
}

func (bvs *internalBVS) shallowEq(other internalBVExpr) bool {
	return bvs.deepEq(other)
}

func (bvs *internalBVS) isLeaf() bool {
	return true
}

func (bvs *internalBVS) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(bvs))
}

/*
 * TY_AND, TY_OR, TY_XOR, TY_ADD, TY_MUL, TY_SDIV, TY_UDIV, TY_SREM, TY_UREM, TY_SHL, TY_LSHR, TY_ASHR
 */

type internalBVExprBinArithmetic struct {
	knd      uint8
	symbol   string
	children []*BVExprPtr
}

func mkBVArithmeticExpr(children []*BVExprPtr, kind int, symbol string) (*internalBVExprBinArithmetic, error) {
	if len(children) < 2 {
		return nil, fmt.Errorf("mkBVArithmeticExpr(): not enough children")
	}
	for i := 1; i < len(children); i++ {
		if children[i].Size() != children[0].Size() {
			return nil, fmt.Errorf("mkBVArithmeticExpr(): invalid sizes")
		}
	}
	return &internalBVExprBinArithmetic{knd: uint8(kind), symbol: symbol, children: children}, nil
}

func (e *internalBVExprBinArithmetic) String() string {
	b := strings.Builder{}
	if e.children[0].e.isLeaf() {
		b.WriteString(e.children[0].String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.children[0].String()))
	}
	for i := 1; i < len(e.children); i++ {
		if e.children[i].e.isLeaf() {
			b.WriteString(fmt.Sprintf(" %s %s", e.symbol, e.children[i].String()))
		} else {
			b.WriteString(fmt.Sprintf(" %s (%s)", e.symbol, e.children[i].String()))
		}
	}
	return b.String()
}

func (e *internalBVExprBinArithmetic) size() uint {
	return e.children[0].Size()
}

func (e *internalBVExprBinArithmetic) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	for i := 0; i < len(e.children); i++ {
		res = append(res, e.children[i].e)
	}
	return res
}

func (e *internalBVExprBinArithmetic) kind() int {
	return int(e.knd)
}

func (e *internalBVExprBinArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))
	for i := 0; i < len(e.children); i++ {
		raw := make([]byte, 8)
		binary.BigEndian.PutUint64(raw, uint64(e.children[i].e.rawPtr()))
		h.Write(raw)
	}
	return h.Sum64()
}

func (e *internalBVExprBinArithmetic) deepEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprBinArithmetic)
	if len(oe.children) != len(e.children) {
		return false
	}
	for i := 0; i < len(e.children); i++ {
		if !e.children[i].e.deepEq(oe.children[i].e) {
			return false
		}
	}
	return true
}

func (e *internalBVExprBinArithmetic) shallowEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprBinArithmetic)
	if len(oe.children) != len(e.children) {
		return false
	}
	for i := 0; i < len(e.children); i++ {
		if e.children[i].e.rawPtr() != oe.children[i].e.rawPtr() {
			return false
		}
	}
	return true
}

func (e *internalBVExprBinArithmetic) isLeaf() bool {
	return false
}

func (e *internalBVExprBinArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkinternalBVExprAnd(children []*BVExprPtr) (*internalBVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_AND, "&")
}
func mkinternalBVExprOr(children []*BVExprPtr) (*internalBVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_OR, "|")
}
func mkinternalBVExprXor(children []*BVExprPtr) (*internalBVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_XOR, "^")
}
func mkinternalBVExprAdd(children []*BVExprPtr) (*internalBVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_ADD, "+")
}
func mkinternalBVExprMul(children []*BVExprPtr) (*internalBVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_MUL, "*")
}
func mkinternalBVExprSdiv(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_SDIV, "s/")
}
func mkinternalBVExprUdiv(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_UDIV, "u/")
}
func mkinternalBVExprSrem(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_SREM, "s%")
}
func mkinternalBVExprUrem(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_UREM, "u%")
}
func mkinternalBVExprShl(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_SHL, "<<")
}
func mkinternalBVExprLshr(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_LSHR, "l>>")
}
func mkinternalBVExprAshr(lhs, rhs *BVExprPtr) (*internalBVExprBinArithmetic, error) {
	children := make([]*BVExprPtr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_ASHR, "a>>")
}

/*
 * TY_NOT, TY_NEG
 */

type internalBVExprUnArithmetic struct {
	knd    uint8
	symbol string
	child  *BVExprPtr
}

func mkinternalBVExprUnArithmetic(child *BVExprPtr, kind int, symbol string) (*internalBVExprUnArithmetic, error) {
	return &internalBVExprUnArithmetic{knd: uint8(kind), symbol: symbol, child: child}, nil
}

func (e *internalBVExprUnArithmetic) String() string {
	b := strings.Builder{}
	if e.child.e.isLeaf() {
		b.WriteString(fmt.Sprintf("%s%s", e.symbol, e.child.String()))
	} else {
		b.WriteString(fmt.Sprintf("%s(%s)", e.symbol, e.child.String()))
	}
	return b.String()
}

func (e *internalBVExprUnArithmetic) size() uint {
	return e.child.Size()
}

func (e *internalBVExprUnArithmetic) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	res = append(res, e.child.e)
	return res
}

func (e *internalBVExprUnArithmetic) kind() int {
	return int(e.knd)
}

func (e *internalBVExprUnArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.e.rawPtr()))
	h.Write(raw)
	return h.Sum64()
}

func (e *internalBVExprUnArithmetic) deepEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprUnArithmetic)
	return e.child.e.deepEq(oe.child.e)
}

func (e *internalBVExprUnArithmetic) shallowEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprUnArithmetic)
	return e.child.e.rawPtr() == oe.child.e.rawPtr()
}

func (e *internalBVExprUnArithmetic) isLeaf() bool {
	return false
}

func (e *internalBVExprUnArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkinternalBVExprNot(e *BVExprPtr) (*internalBVExprUnArithmetic, error) {
	return mkinternalBVExprUnArithmetic(e, TY_NOT, "~")
}
func mkinternalBVExprNeg(e *BVExprPtr) (*internalBVExprUnArithmetic, error) {
	return mkinternalBVExprUnArithmetic(e, TY_NEG, "-")
}

/*
 * TY_ULT, TY_ULE, TY_UGT, TY_UGE, TY_SLT, TY_SLE, TY_SGT, TY_SGE, TY_EQ
 */

type internalBoolExprCmp struct {
	knd      uint8
	symbol   string
	lhs, rhs *BVExprPtr
}

func mkinternalBoolExprCmp(lhs, rhs *BVExprPtr, kind int, symbol string) (*internalBoolExprCmp, error) {
	if rhs.Size() != lhs.Size() {
		return nil, fmt.Errorf("mkinternalBoolExprCmp(): invalid sizes")
	}
	return &internalBoolExprCmp{knd: uint8(kind), symbol: symbol, lhs: lhs, rhs: rhs}, nil
}

func (e *internalBoolExprCmp) String() string {
	b := strings.Builder{}
	if e.lhs.e.isLeaf() {
		b.WriteString(e.lhs.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.lhs.String()))
	}

	b.WriteString(fmt.Sprintf(" %s ", e.symbol))

	if e.rhs.e.isLeaf() {
		b.WriteString(e.rhs.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.rhs.String()))
	}
	return b.String()
}

func (e *internalBoolExprCmp) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	res = append(res, e.lhs.e)
	res = append(res, e.rhs.e)
	return res
}

func (e *internalBoolExprCmp) kind() int {
	return int(e.knd)
}

func (e *internalBoolExprCmp) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.lhs.e.rawPtr()))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.rhs.e.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *internalBoolExprCmp) deepEq(other internalBoolExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBoolExprCmp)
	if !e.lhs.e.deepEq(oe.lhs.e) {
		return false
	}
	if !e.rhs.e.deepEq(oe.rhs.e) {
		return false
	}
	return true
}

func (e *internalBoolExprCmp) shallowEq(other internalBoolExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBoolExprCmp)
	if e.lhs.e.rawPtr() != oe.lhs.e.rawPtr() {
		return false
	}
	if e.rhs.e.rawPtr() != oe.rhs.e.rawPtr() {
		return false
	}
	return true
}

func (e *internalBoolExprCmp) isLeaf() bool {
	return false
}

func (e *internalBoolExprCmp) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkinternalBoolExprUlt(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_ULT, "u<")
}
func mkinternalBoolExprUle(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_ULE, "u<=")
}
func mkinternalBoolExprUgt(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_UGT, "u>")
}
func mkinternalBoolExprUge(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_UGE, "u>=")
}
func mkinternalBoolExprSlt(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_SLT, "s<")
}
func mkinternalBoolExprSle(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_SLE, "s<=")
}
func mkinternalBoolExprSgt(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_SGT, "s>")
}
func mkinternalBoolExprSge(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_SGE, "s>=")
}
func mkinternalBoolExprEq(lhs, rhs *BVExprPtr) (*internalBoolExprCmp, error) {
	return mkinternalBoolExprCmp(lhs, rhs, TY_EQ, "==")
}

/*
 * TY_BOOL_AND, TY_BOOL_OR
 */

type internalBoolExprNaryOp struct {
	knd      uint8
	symbol   string
	children []*BoolExprPtr
}

func mkinternalBoolExprNaryOp(children []*BoolExprPtr, kind int, symbol string) (*internalBoolExprNaryOp, error) {
	return &internalBoolExprNaryOp{knd: uint8(kind), symbol: symbol, children: children}, nil
}

func (e *internalBoolExprNaryOp) String() string {
	b := strings.Builder{}
	if e.children[0].e.isLeaf() {
		b.WriteString(e.children[0].e.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.children[0].e.String()))
	}

	for i := 1; i < len(e.children); i++ {
		b.WriteString(fmt.Sprintf(" %s ", e.symbol))
		if e.children[i].e.isLeaf() {
			b.WriteString(e.children[i].String())
		} else {
			b.WriteString(fmt.Sprintf("(%s)", e.children[i].String()))
		}
	}
	return b.String()
}

func (e *internalBoolExprNaryOp) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	for i := 0; i < len(e.children); i++ {
		res = append(res, e.children[i].e)
	}
	return res
}

func (e *internalBoolExprNaryOp) kind() int {
	return int(e.knd)
}

func (e *internalBoolExprNaryOp) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))

	for i := 0; i < len(e.children); i++ {
		raw := make([]byte, 8)
		binary.BigEndian.PutUint64(raw, uint64(e.children[i].e.rawPtr()))
		h.Write(raw)
	}
	return h.Sum64()
}

func (e *internalBoolExprNaryOp) deepEq(other internalBoolExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBoolExprNaryOp)
	if len(e.children) != len(oe.children) {
		return false
	}

	for i := 0; i < len(e.children); i++ {
		if !e.children[i].e.deepEq(oe.children[i].e) {
			return false
		}
	}
	return true
}

func (e *internalBoolExprNaryOp) shallowEq(other internalBoolExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBoolExprNaryOp)
	if len(e.children) != len(oe.children) {
		return false
	}

	for i := 0; i < len(e.children); i++ {
		if e.children[i].e.rawPtr() != oe.children[i].e.rawPtr() {
			return false
		}
	}
	return true
}

func (e *internalBoolExprNaryOp) isLeaf() bool {
	return false
}

func (e *internalBoolExprNaryOp) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkinternalBoolExprAnd(children []*BoolExprPtr) (*internalBoolExprNaryOp, error) {
	return mkinternalBoolExprNaryOp(children, TY_BOOL_AND, "&&")
}
func mkinternalBoolExprOr(children []*BoolExprPtr) (*internalBoolExprNaryOp, error) {
	return mkinternalBoolExprNaryOp(children, TY_BOOL_OR, "||")
}

/*
 * TY_BOOL_NOT
 */

type internalBoolUnArithmetic struct {
	knd    uint8
	symbol string
	child  *BoolExprPtr
}

func mkinternalBoolUnArithmetic(child *BoolExprPtr, kind int, symbol string) (*internalBoolUnArithmetic, error) {
	return &internalBoolUnArithmetic{knd: uint8(kind), symbol: symbol, child: child}, nil
}

func (e *internalBoolUnArithmetic) String() string {
	b := strings.Builder{}
	if e.child.e.isLeaf() {
		b.WriteString(fmt.Sprintf("%s%s", e.symbol, e.child.String()))
	} else {
		b.WriteString(fmt.Sprintf("%s(%s)", e.symbol, e.child.String()))
	}
	return b.String()
}

func (e *internalBoolUnArithmetic) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	res = append(res, e.child.e)
	return res
}

func (e *internalBoolUnArithmetic) kind() int {
	return int(e.knd)
}

func (e *internalBoolUnArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.e.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *internalBoolUnArithmetic) deepEq(other internalBoolExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBoolUnArithmetic)
	return e.child.e.deepEq(oe.child.e)
}

func (e *internalBoolUnArithmetic) shallowEq(other internalBoolExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBoolUnArithmetic)
	return e.child.e.rawPtr() != oe.child.e.rawPtr()
}

func (e *internalBoolUnArithmetic) isLeaf() bool {
	return false
}

func (e *internalBoolUnArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkinternalBoolNot(e *BoolExprPtr) (*internalBoolUnArithmetic, error) {
	return mkinternalBoolUnArithmetic(e, TY_BOOL_NOT, "!")
}

/*
 *  TY_EXTRACT
 */

type internalBVExprExtract struct {
	child     *BVExprPtr
	high, low uint
}

func mkinternalBVExprExtract(child *BVExprPtr, high, low uint) (*internalBVExprExtract, error) {
	if high < low {
		return nil, fmt.Errorf("mkinternalBVExprExtract(): high < low")
	}
	if child.Size() < high-low+1 {
		return nil, fmt.Errorf("mkinternalBVExprExtract(): high-low+1 > child.Size")
	}
	return &internalBVExprExtract{child: child, high: high, low: low}, nil
}

func (e *internalBVExprExtract) String() string {
	b := strings.Builder{}
	if e.child.e.isLeaf() {
		b.WriteString(e.child.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.child.String()))
	}
	b.WriteString(fmt.Sprintf("[%d:%d]", e.high, e.low))
	return b.String()
}

func (e *internalBVExprExtract) size() uint {
	return e.high - e.low + 1
}

func (e *internalBVExprExtract) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	res = append(res, e.child.e)
	return res
}

func (e *internalBVExprExtract) kind() int {
	return TY_EXTRACT
}

func (e *internalBVExprExtract) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte("TY_EXTRACT"))
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.e.rawPtr()))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.low))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.high))
	h.Write(raw)
	return h.Sum64()
}

func (e *internalBVExprExtract) deepEq(other internalBVExpr) bool {
	if other.kind() != TY_EXTRACT {
		return false
	}
	oe := other.(*internalBVExprExtract)
	return e.child.e.deepEq(oe.child.e) &&
		e.low == oe.low &&
		e.high == oe.high
}

func (e *internalBVExprExtract) shallowEq(other internalBVExpr) bool {
	if other.kind() != TY_EXTRACT {
		return false
	}
	oe := other.(*internalBVExprExtract)
	return e.child.e.rawPtr() == oe.child.e.rawPtr() &&
		e.low == oe.low &&
		e.high == oe.high
}

func (e *internalBVExprExtract) isLeaf() bool {
	return false
}

func (e *internalBVExprExtract) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

/*
 *  TY_CONCAT
 */

type internalBVExprConcat struct {
	children []*BVExprPtr
}

func mkinternalBVExprConcat(children []*BVExprPtr) (*internalBVExprConcat, error) {
	if len(children) < 2 {
		return nil, fmt.Errorf("mkinternalBVExprConcat(): expected at least 2 children")
	}
	return &internalBVExprConcat{children: children}, nil
}

func (e *internalBVExprConcat) String() string {
	b := strings.Builder{}
	if e.children[0].e.isLeaf() {
		b.WriteString(e.children[0].String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.children[0].String()))
	}

	for i := 1; i < len(e.children); i++ {
		if e.children[i].e.isLeaf() {
			b.WriteString(fmt.Sprintf(" .. %s", e.children[i].String()))
		} else {
			b.WriteString(fmt.Sprintf(" .. (%s)", e.children[i].String()))
		}
	}
	return b.String()
}

func (e *internalBVExprConcat) size() uint {
	size := uint(0)
	for i := 0; i < len(e.children); i++ {
		size += e.children[i].Size()
	}
	return size
}

func (e *internalBVExprConcat) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	for i := 0; i < len(e.children); i++ {
		res = append(res, e.children[i].e)
	}
	return res
}

func (e *internalBVExprConcat) kind() int {
	return TY_CONCAT
}

func (e *internalBVExprConcat) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte("TY_CONCAT"))
	for i := 0; i < len(e.children); i++ {
		raw := make([]byte, 8)
		binary.BigEndian.PutUint64(raw, uint64(e.children[i].e.rawPtr()))
		h.Write(raw)
	}
	return h.Sum64()
}

func (e *internalBVExprConcat) deepEq(other internalBVExpr) bool {
	if other.kind() != TY_CONCAT {
		return false
	}
	oe := other.(*internalBVExprConcat)
	if len(e.children) != len(oe.children) {
		return false
	}
	for i := 0; i < len(e.children); i++ {
		if !e.children[i].e.deepEq(oe.children[i].e) {
			return false
		}
	}
	return true
}

func (e *internalBVExprConcat) shallowEq(other internalBVExpr) bool {
	if other.kind() != TY_CONCAT {
		return false
	}
	oe := other.(*internalBVExprConcat)
	if len(e.children) != len(oe.children) {
		return false
	}
	for i := 0; i < len(e.children); i++ {
		if e.children[i].e.rawPtr() != oe.children[i].e.rawPtr() {
			return false
		}
	}
	return true
}

func (e *internalBVExprConcat) isLeaf() bool {
	return false
}

func (e *internalBVExprConcat) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

/*
 *   TY_ZEXT, TY_SEXT
 */

type internalBVExprExtend struct {
	signed bool
	n      uint
	child  *BVExprPtr
}

func mkinternalBVExprExtend(child *BVExprPtr, signed bool, n uint) (*internalBVExprExtend, error) {
	if n == 0 {
		return nil, fmt.Errorf("trying to create a BVExpreExtend with n == 0")
	}
	return &internalBVExprExtend{child: child, n: n, signed: signed}, nil
}

func (e *internalBVExprExtend) String() string {
	b := strings.Builder{}
	if e.signed {
		b.WriteString("SExt(")
	} else {
		b.WriteString("ZExt(")
	}
	if e.child.e.isLeaf() {
		b.WriteString(fmt.Sprintf("%s, ", e.child.String()))
	} else {
		b.WriteString(fmt.Sprintf("(%s), ", e.child.String()))
	}
	b.WriteString(fmt.Sprintf("%d)", e.n))
	return b.String()
}

func (e *internalBVExprExtend) size() uint {
	return e.child.Size() + e.n
}

func (e *internalBVExprExtend) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	res = append(res, e.child.e)
	return res
}

func (e *internalBVExprExtend) kind() int {
	if e.signed {
		return TY_SEXT
	}
	return TY_ZEXT
}

func (e *internalBVExprExtend) hash() uint64 {
	h := xxhash.New()
	if e.signed {
		h.Write([]byte("TY_SEXT"))
	} else {
		h.Write([]byte("TY_ZEXT"))
	}

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.e.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *internalBVExprExtend) deepEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprExtend)
	return e.n == oe.n && e.child.e.deepEq(oe.child.e)
}

func (e *internalBVExprExtend) shallowEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprExtend)
	return e.n == oe.n && e.child.e.rawPtr() == oe.child.e.rawPtr()
}

func (e *internalBVExprExtend) isLeaf() bool {
	return false
}

func (e *internalBVExprExtend) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkinternalBVExprSExt(e *BVExprPtr, n uint) (*internalBVExprExtend, error) {
	return mkinternalBVExprExtend(e, true, n)
}
func mkinternalBVExprZExt(e *BVExprPtr, n uint) (*internalBVExprExtend, error) {
	return mkinternalBVExprExtend(e, false, n)
}

/*
 *   TY_ITE
 */

type internalBVExprITE struct {
	cond    *BoolExprPtr
	iftrue  *BVExprPtr
	iffalse *BVExprPtr
}

func mkinternalBVExprITE(cond *BoolExprPtr, iftrue *BVExprPtr, iffalse *BVExprPtr) (*internalBVExprITE, error) {
	if iftrue.Size() != iffalse.Size() {
		return nil, fmt.Errorf("mkinternalBVExprITE(): invalid sizes")
	}
	return &internalBVExprITE{cond: cond, iftrue: iftrue, iffalse: iffalse}, nil
}

func (e *internalBVExprITE) String() string {
	b := strings.Builder{}
	b.WriteString("ITE(")
	b.WriteString(e.cond.String())
	b.WriteString(", ")
	b.WriteString(e.iftrue.String())
	b.WriteString(", ")
	b.WriteString(e.iffalse.String())
	b.WriteString(")")
	return b.String()
}

func (e *internalBVExprITE) size() uint {
	return e.iftrue.Size()
}

func (e *internalBVExprITE) subexprs() []internalExpr {
	res := make([]internalExpr, 0)
	res = append(res, e.iftrue.e)
	res = append(res, e.iffalse.e)
	res = append(res, e.cond.e)
	return res
}

func (e *internalBVExprITE) kind() int {
	return TY_ITE
}

func (e *internalBVExprITE) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte("TY_ITE"))

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.cond.e.rawPtr()))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.iftrue.e.rawPtr()))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.iffalse.e.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *internalBVExprITE) deepEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprITE)
	return e.cond.e.deepEq(oe.cond.e) && e.iftrue.e.deepEq(oe.iftrue.e) && e.iffalse.e.deepEq(oe.iffalse.e)
}

func (e *internalBVExprITE) shallowEq(other internalBVExpr) bool {
	if other.kind() != e.kind() {
		return false
	}
	oe := other.(*internalBVExprITE)
	return e.cond.e.rawPtr() == oe.cond.e.rawPtr() &&
		e.iftrue.e.rawPtr() == oe.iftrue.e.rawPtr() &&
		e.iffalse.e.rawPtr() == oe.iffalse.e.rawPtr()
}

func (e *internalBVExprITE) isLeaf() bool {
	return false
}

func (e *internalBVExprITE) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}
