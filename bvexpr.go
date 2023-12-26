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

type Expr interface {
	Kind() int
	String() string
	DeepEq(Expr) bool
	Children() []Expr

	isLeaf() bool
	rawPtr() uintptr
	hash() uint64
}

type BVExpr interface {
	Expr

	Size() uint
}

type BoolExpr interface {
	Expr

	IsTrue() bool
	IsFalse() bool
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

func (bvv *BVV) isLeaf() bool {
	return true
}

func (bvv *BVV) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(bvv))
}

/*
 *  TY_BOOL_CONST
 */

type BoolVal struct {
	Value BoolConst
}

func mkBoolConst(value bool) *BoolVal {
	if value {
		return &BoolVal{Value: BoolTrue()}
	}
	return &BoolVal{Value: BoolFalse()}
}

func (b *BoolVal) IsTrue() bool {
	return b.Value.Value
}

func (b *BoolVal) IsFalse() bool {
	return !b.Value.Value
}

func (b *BoolVal) String() string {
	return b.Value.String()
}

func (b *BoolVal) Children() []Expr {
	return make([]Expr, 0)
}

func (b *BoolVal) Kind() int {
	return TY_BOOL_CONST
}

func (b *BoolVal) hash() uint64 {
	if b.Value.Value {
		return 1
	}
	return 0
}

func (b *BoolVal) DeepEq(other Expr) bool {
	if other.Kind() != TY_BOOL_CONST {
		return false
	}
	ob := other.(*BoolVal)
	return ob.Value.Value == b.Value.Value
}

func (b *BoolVal) isLeaf() bool {
	return true
}

func (b *BoolVal) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(b))
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

func (bvs *BVS) isLeaf() bool {
	return true
}

func (bvs *BVS) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(bvs))
}

/*
 * TY_AND, TY_OR, TY_XOR, TY_ADD, TY_MUL, TY_SDIV, TY_UDIV, TY_SREM, TY_UREM, TY_SHL, TY_LSHR, TY_ASHR
 */

type BVExprBinArithmetic struct {
	kind     int
	symbol   string
	children []BVExpr
}

func mkBVArithmeticExpr(children []BVExpr, kind int, symbol string) (*BVExprBinArithmetic, error) {
	if len(children) < 2 {
		return nil, fmt.Errorf("mkBVArithmeticExpr(): not enough children")
	}
	for i := 1; i < len(children); i++ {
		if children[i].Size() != children[0].Size() {
			return nil, fmt.Errorf("mkBVArithmeticExpr(): invalid sizes")
		}
	}
	return &BVExprBinArithmetic{kind: kind, symbol: symbol, children: children}, nil
}

func (e *BVExprBinArithmetic) String() string {
	b := strings.Builder{}
	if e.children[0].isLeaf() {
		b.WriteString(e.children[0].String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.children[0].String()))
	}
	for i := 1; i < len(e.children); i++ {
		if e.children[i].isLeaf() {
			b.WriteString(fmt.Sprintf(" %s %s", e.symbol, e.children[i]))
		} else {
			b.WriteString(fmt.Sprintf(" %s (%s)", e.symbol, e.children[i]))
		}
	}
	return b.String()
}

func (e *BVExprBinArithmetic) Size() uint {
	return e.children[0].Size()
}

func (e *BVExprBinArithmetic) Children() []Expr {
	res := make([]Expr, 0)
	for i := 0; i < len(e.children); i++ {
		res = append(res, e.children[i])
	}
	return res
}

func (e *BVExprBinArithmetic) Kind() int {
	return e.kind
}

func (e *BVExprBinArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))
	for i := 0; i < len(e.children); i++ {
		raw := make([]byte, 8)
		binary.BigEndian.PutUint64(raw, uint64(e.children[i].rawPtr()))
		h.Write(raw)
	}
	return h.Sum64()
}

func (e *BVExprBinArithmetic) DeepEq(other Expr) bool {
	if other.Kind() != e.kind {
		return false
	}
	oe := other.(*BVExprBinArithmetic)
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

func (e *BVExprBinArithmetic) isLeaf() bool {
	return false
}

func (e *BVExprBinArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkBVExprAnd(children []BVExpr) (*BVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_AND, "&")
}
func mkBVExprOr(children []BVExpr) (*BVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_OR, "|")
}
func mkBVExprXor(children []BVExpr) (*BVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_XOR, "^")
}
func mkBVExprAdd(children []BVExpr) (*BVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_ADD, "+")
}
func mkBVExprMul(children []BVExpr) (*BVExprBinArithmetic, error) {
	return mkBVArithmeticExpr(children, TY_MUL, "*")
}
func mkBVExprSdiv(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_SDIV, "s/")
}
func mkBVExprUdiv(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_UDIV, "u/")
}
func mkBVExprSrem(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_SREM, "s%")
}
func mkBVExprUrem(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_UREM, "u%")
}
func mkBVExprShl(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_SHL, "<<")
}
func mkBVExprLshr(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_LSHR, "l>>")
}
func mkBVExprAshr(lhs, rhs BVExpr) (*BVExprBinArithmetic, error) {
	children := make([]BVExpr, 0)
	children = append(children, lhs)
	children = append(children, rhs)
	return mkBVArithmeticExpr(children, TY_ASHR, "a>>")
}

/*
 * TY_NOT, TY_NEG
 */

type BVExprUnArithmetic struct {
	kind   int
	symbol string
	child  BVExpr
}

func mkBVExprUnArithmetic(child BVExpr, kind int, symbol string) (*BVExprUnArithmetic, error) {
	return &BVExprUnArithmetic{kind: kind, symbol: symbol, child: child}, nil
}

func (e *BVExprUnArithmetic) String() string {
	b := strings.Builder{}
	if e.child.isLeaf() {
		b.WriteString(fmt.Sprintf("%s%s", e.symbol, e.child.String()))
	} else {
		b.WriteString(fmt.Sprintf("%s(%s)", e.symbol, e.child.String()))
	}
	return b.String()
}

func (e *BVExprUnArithmetic) Size() uint {
	return e.child.Size()
}

func (e *BVExprUnArithmetic) Children() []Expr {
	res := make([]Expr, 0)
	res = append(res, e.child)
	return res
}

func (e *BVExprUnArithmetic) Kind() int {
	return e.kind
}

func (e *BVExprUnArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.rawPtr()))
	h.Write(raw)
	return h.Sum64()
}

func (e *BVExprUnArithmetic) DeepEq(other Expr) bool {
	if other.Kind() != e.kind {
		return false
	}
	oe := other.(*BVExprUnArithmetic)
	return e.child.DeepEq(oe.child)
}

func (e *BVExprUnArithmetic) isLeaf() bool {
	return false
}

func (e *BVExprUnArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkBVExprNot(e BVExpr) (*BVExprUnArithmetic, error) {
	return mkBVExprUnArithmetic(e, TY_NOT, "~")
}
func mkBVExprNeg(e BVExpr) (*BVExprUnArithmetic, error) {
	return mkBVExprUnArithmetic(e, TY_NEG, "-")
}

/*
 * TY_ULT, TY_ULE, TY_UGT, TY_UGE, TY_SLT, TY_SLE, TY_SGT, TY_SGE, TY_EQ
 */

type BoolExprCmp struct {
	kind     int
	symbol   string
	lhs, rhs BVExpr
}

func mkBoolExprCmp(lhs, rhs BVExpr, kind int, symbol string) (*BoolExprCmp, error) {
	if rhs.Size() != lhs.Size() {
		return nil, fmt.Errorf("mkBoolExprCmp(): invalid sizes")
	}
	return &BoolExprCmp{kind: kind, symbol: symbol, lhs: lhs, rhs: rhs}, nil
}

func (e *BoolExprCmp) IsTrue() bool {
	return false
}

func (e *BoolExprCmp) IsFalse() bool {
	return false
}

func (e *BoolExprCmp) String() string {
	b := strings.Builder{}
	if e.lhs.isLeaf() {
		b.WriteString(e.lhs.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.lhs.String()))
	}

	b.WriteString(fmt.Sprintf(" %s ", e.symbol))

	if e.rhs.isLeaf() {
		b.WriteString(e.rhs.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.rhs.String()))
	}
	return b.String()
}

func (e *BoolExprCmp) Children() []Expr {
	res := make([]Expr, 0)
	res = append(res, e.lhs)
	res = append(res, e.rhs)
	return res
}

func (e *BoolExprCmp) Kind() int {
	return e.kind
}

func (e *BoolExprCmp) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.lhs.rawPtr()))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.rhs.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *BoolExprCmp) DeepEq(other Expr) bool {
	if other.Kind() != e.kind {
		return false
	}
	oe := other.(*BoolExprCmp)
	if !e.lhs.DeepEq(oe.lhs) {
		return false
	}
	if !e.rhs.DeepEq(oe.rhs) {
		return false
	}
	return true
}

func (e *BoolExprCmp) isLeaf() bool {
	return false
}

func (e *BoolExprCmp) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkBoolExprUlt(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_ULT, "u<")
}
func mkBoolExprUle(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_ULE, "u<=")
}
func mkBoolExprUgt(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_UGT, "u>")
}
func mkBoolExprUge(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_UGE, "u>=")
}
func mkBoolExprSlt(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_SLT, "s<")
}
func mkBoolExprSle(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_SLE, "s<=")
}
func mkBoolExprSgt(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_SGT, "s>")
}
func mkBoolExprSge(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_SGE, "s>=")
}
func mkBoolExprEq(lhs, rhs BVExpr) (*BoolExprCmp, error) {
	return mkBoolExprCmp(lhs, rhs, TY_EQ, "==")
}

/*
 * TY_BOOL_AND, TY_BOOL_OR
 */

type BoolBinArithmetic struct {
	kind     int
	symbol   string
	lhs, rhs BoolExpr
}

func mkBoolBinArithmetic(lhs, rhs BoolExpr, kind int, symbol string) (*BoolBinArithmetic, error) {
	return &BoolBinArithmetic{kind: kind, symbol: symbol, lhs: lhs, rhs: rhs}, nil
}

func (e *BoolBinArithmetic) IsTrue() bool {
	return false
}

func (e *BoolBinArithmetic) IsFalse() bool {
	return false
}

func (e *BoolBinArithmetic) String() string {
	b := strings.Builder{}
	if e.lhs.isLeaf() {
		b.WriteString(e.lhs.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.lhs.String()))
	}

	b.WriteString(fmt.Sprintf(" %s ", e.symbol))

	if e.rhs.isLeaf() {
		b.WriteString(e.rhs.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.rhs.String()))
	}
	return b.String()
}

func (e *BoolBinArithmetic) Children() []Expr {
	res := make([]Expr, 0)
	res = append(res, e.lhs)
	res = append(res, e.rhs)
	return res
}

func (e *BoolBinArithmetic) Kind() int {
	return e.kind
}

func (e *BoolBinArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.lhs.rawPtr()))
	h.Write(raw)
	binary.BigEndian.PutUint64(raw, uint64(e.rhs.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *BoolBinArithmetic) DeepEq(other Expr) bool {
	if other.Kind() != e.kind {
		return false
	}
	oe := other.(*BoolExprCmp)
	if !e.lhs.DeepEq(oe.lhs) {
		return false
	}
	if !e.rhs.DeepEq(oe.rhs) {
		return false
	}
	return true
}

func (e *BoolBinArithmetic) isLeaf() bool {
	return false
}

func (e *BoolBinArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkBoolAnd(lhs, rhs BoolExpr) (*BoolBinArithmetic, error) {
	return mkBoolBinArithmetic(lhs, rhs, TY_BOOL_AND, "&&")
}
func mkBoolOr(lhs, rhs BoolExpr) (*BoolBinArithmetic, error) {
	return mkBoolBinArithmetic(lhs, rhs, TY_BOOL_OR, "||")
}

/*
 * TY_BOOL_NOT
 */

type BoolUnArithmetic struct {
	kind   int
	symbol string
	child  BoolExpr
}

func mkBoolUnArithmetic(child BoolExpr, kind int, symbol string) (*BoolUnArithmetic, error) {
	return &BoolUnArithmetic{kind: kind, symbol: symbol, child: child}, nil
}

func (e *BoolUnArithmetic) IsTrue() bool {
	return false
}

func (e *BoolUnArithmetic) IsFalse() bool {
	return false
}

func (e *BoolUnArithmetic) String() string {
	b := strings.Builder{}
	if e.child.isLeaf() {
		b.WriteString(fmt.Sprintf("%s%s", e.symbol, e.child.String()))
	} else {
		b.WriteString(fmt.Sprintf("%s(%s)", e.symbol, e.child.String()))
	}
	return b.String()
}

func (e *BoolUnArithmetic) Children() []Expr {
	res := make([]Expr, 0)
	res = append(res, e.child)
	return res
}

func (e *BoolUnArithmetic) Kind() int {
	return e.kind
}

func (e *BoolUnArithmetic) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte(e.symbol))

	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.rawPtr()))
	h.Write(raw)

	return h.Sum64()
}

func (e *BoolUnArithmetic) DeepEq(other Expr) bool {
	if other.Kind() != e.kind {
		return false
	}
	oe := other.(*BoolUnArithmetic)
	return e.child.DeepEq(oe.child)
}

func (e *BoolUnArithmetic) isLeaf() bool {
	return false
}

func (e *BoolUnArithmetic) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

func mkBoolNot(e BoolExpr) (*BoolUnArithmetic, error) {
	return mkBoolUnArithmetic(e, TY_BOOL_AND, "!")
}

/*
 *  TY_EXTRACT
 */

type BVExtract struct {
	child     BVExpr
	high, low uint
}

func mkBVExtract(child BVExpr, high, low uint) (*BVExtract, error) {
	if high < low {
		return nil, fmt.Errorf("mkBVExtract(): high < low")
	}
	if child.Size() < high-low+1 {
		return nil, fmt.Errorf("mkBVExtract(): high-low+1 > child.Size")
	}
	return &BVExtract{child: child, high: high, low: low}, nil
}

func (e *BVExtract) String() string {
	b := strings.Builder{}
	if e.child.isLeaf() {
		b.WriteString(e.child.String())
	} else {
		b.WriteString(fmt.Sprintf("(%s)", e.child.String()))
	}
	b.WriteString(fmt.Sprintf("[%d:%d]", e.high, e.low))
	return b.String()
}

func (e *BVExtract) Size() uint {
	return e.high - e.low + 1
}

func (e *BVExtract) Children() []Expr {
	res := make([]Expr, 0)
	res = append(res, e.child)
	return res
}

func (e *BVExtract) Kind() int {
	return TY_EXTRACT
}

func (e *BVExtract) hash() uint64 {
	h := xxhash.New()
	h.Write([]byte("TY_EXTRACT"))
	raw := make([]byte, 8)
	binary.BigEndian.PutUint64(raw, uint64(e.child.rawPtr()))
	h.Write(raw)
	return h.Sum64()
}

func (e *BVExtract) DeepEq(other Expr) bool {
	if other.Kind() != TY_EXTRACT {
		return false
	}
	oe := other.(*BVExtract)
	return e.child.DeepEq(oe.child)
}

func (e *BVExtract) isLeaf() bool {
	return false
}

func (e *BVExtract) rawPtr() uintptr {
	return uintptr(unsafe.Pointer(e))
}

// TODO: TY_CONCAT, TY_ZEXT, TY_SEXT, TY_ITE
