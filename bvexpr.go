package gosmt

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unsafe"

	"github.com/cespare/xxhash"
)

const (
	TY_SYM     = 1
	TY_CONST   = 2
	TY_EXTRACT = 3
	TY_CONCAT  = 4
	TY_ZEXT    = 5
	TY_SEXT    = 6
	TY_ITE     = 7
	TY_SHL     = 8
	TY_LSHR    = 9
	TY_ASHR    = 10
	TY_NEG     = 11
	TY_NOT     = 12
	TY_AND     = 13
	TY_OR      = 14
	TY_XOR     = 15
	TY_ADD     = 16
	TY_MUL     = 17
	TY_SDIV    = 18
	TY_UDIV    = 19
	TY_SREM    = 20
	TY_UREM    = 21
	TY_ULT     = 22
	TY_ULE     = 23
	TY_UGT     = 24
	TY_UGE     = 25
	TY_SLT     = 26
	TY_SLE     = 27
	TY_SGT     = 28
	TY_SGE     = 29
	TY_EQ      = 30

	TY_BOOL_CONST = 31
	TY_BOOL_NOT   = 32
	TY_BOOL_AND   = 33
	TY_BOOL_OR    = 34
)

type Expr interface {
	Kind() int
	String() string
	DeepEq(Expr) bool
	Children() []Expr
	IsLeaf() bool

	rawPtr() uintptr
	hash() uint64
}

type BVExpr interface {
	Expr

	Size() uint
}

type BoolExpr interface {
	Expr
}

/*
 *  TY_CONST
 */

type BVV struct {
	Value BVConst
}

func mkBVV(value int64, size uint) *BVV {
	return &BVV{Value: *MakeBVConst(value, size)}
}

func (bvv *BVV) String() string {
	return fmt.Sprintf("0x%x", bvv.Value.value)
}

func (bvv *BVV) Size() uint {
	return bvv.Value.Size
}

func (bvv *BVV) Children() []Expr {
	return make([]Expr, 0)
}

func (bvv *BVV) Kind() int {
	return TY_CONST
}

func (bvv *BVV) hash() uint64 {
	if bvv.Value.Size > 64 {
		cpy := bvv.Value.Copy()
		cpy.Truncate(63, 0)
		return cpy.AsULong()
	}
	return bvv.Value.AsULong()
}

func (bvv *BVV) DeepEq(other Expr) bool {
	if other.Kind() != TY_CONST {
		return false
	}
	obvv := other.(*BVV)
	res, err := bvv.Value.Eq(&obvv.Value)
	if err != nil || !res.Value {
		return false
	}
	return true
}

func (bvv *BVV) IsLeaf() bool {
	return true
}

func (bvv *BVV) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(bvv))
}

/*
 *  TY_SYM
 */

type BVS struct {
	Name string
	size uint
}

func mkBVS(name string, size uint) *BVS {
	return &BVS{Name: name, size: size}
}

func (bvs *BVS) String() string {
	return bvs.Name
}

func (bvs *BVS) Size() uint {
	return bvs.size
}

func (bvs *BVS) Children() []Expr {
	return make([]Expr, 0)
}

func (bvs *BVS) Kind() int {
	return TY_SYM
}

func (bvs *BVS) hash() uint64 {
	h := xxhash.New()
	n, err := h.Write([]byte(bvs.Name))
	if err != nil || n != len(bvs.Name) {
		panic(err)
	}
	return h.Sum64()
}

func (bvs *BVS) DeepEq(other Expr) bool {
	if other.Kind() != TY_SYM {
		return false
	}
	obvs := other.(*BVS)
	return obvs.size == bvs.size && obvs.Name == bvs.Name
}

func (bvs *BVS) IsLeaf() bool {
	return true
}

func (bvs *BVS) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(bvs))
}

/*
 * TY_AND, TY_OR, TY_XOR, TY_ADD, TY_MUL, TY_SDIV, TY_UDIV, TY_SREM, TY_UREM
 */

type BVExprArithmetic struct {
	kind     int
	symbol   string
	children []BVExpr
}

func mkBVArithmeticExpr(children []BVExpr, kind int, symbol string) (*BVExprArithmetic, error) {
	if len(children) < 2 {
		return nil, fmt.Errorf("mkBVArithmeticExpr(): not enough children")
	}
	for i := 1; i < len(children); i++ {
		if children[i].Size() != children[0].Size() {
			return nil, fmt.Errorf("mkBVArithmeticExpr(): invalid sizes")
		}
	}
	return &BVExprArithmetic{kind: kind, symbol: symbol, children: children}, nil
}

func (e *BVExprArithmetic) String() string {
	b := strings.Builder{}
	if e.children[0].IsLeaf() {
		b.WriteString(e.children[0].String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.children[0].String()))
	}
	for i := 1; i < len(e.children); i++ {
		if e.children[i].IsLeaf() {
			b.WriteString(fmt.Sprintf(" %s %s", e.symbol, e.children[i]))
		} else {
			b.WriteString(fmt.Sprintf(" %s (%s)", e.symbol, e.children[i]))
		}
	}
	return b.String()
}

func (e *BVExprArithmetic) Size() uint {
	return e.children[0].Size()
}

func (e *BVExprArithmetic) Children() []Expr {
	res := make([]Expr, 0)
	for i := 0; i < len(e.children); i++ {
		res = append(res, e.children[i])
	}
	return res
}

func (e *BVExprArithmetic) Kind() int {
	return e.kind
}

func (e *BVExprArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))
	for i := 0; i < len(e.children); i++ {
		raw := make([]byte, 8)
		binary.BigEndian.PutUint64(raw, uint64(e.children[i].rawPtr()))
		h.Write(raw)
	}
	return h.Sum64()
}

func (e *BVExprArithmetic) DeepEq(other Expr) bool {
	if other.Kind() != TY_ADD {
		return false
	}
	oe := other.(*BVExprArithmetic)
	if len(oe.children) != len(e.children) {
		return false
	}
	for i := 0; i < len(e.children); i++ {
		if !e.children[i].DeepEq(oe.children[i]) {
			return false
		}
	}
	return true
}

func (e *BVExprArithmetic) IsLeaf() bool {
	return false
}

func (e *BVExprArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkBVExprAnd(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_AND, "&")
}

func mkBVExprOr(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_OR, "|")
}
func mkBVExprXor(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_XOR, "^")
}
func mkBVExprAdd(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_ADD, "+")
}
func mkBVExprMul(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_MUL, "*")
}
func mkBVExprSdiv(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_SDIV, "s/")
}
func mkBVExprUdiv(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_UDIV, "u/")
}
func mkBVExprSrem(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_SREM, "s%")
}
func mkBVExprUrem(children []BVExpr) (*BVExprArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_UREM, "u%")
}
