package gosmt

import (
	"fmt"
	"math/big"
)

var zero = big.NewInt(0)
var one = big.NewInt(1)

type BVConst struct {
	Size  uint
	mask  *big.Int
	value *big.Int
}

func makeMask(size uint) *big.Int {
	bytes := make([]byte, size/8)
	for i := uint(0); i < size/8; i++ {
		bytes[i] = 0xff
	}
	v := big.NewInt(0)
	v.SetBytes(bytes)
	for i := size / 8 * 8; i < size/8*8+size%8; i++ {
		v.SetBit(v, int(i), 1)
	}
	return v
}

func MakeBVConst(value int64, size uint) *BVConst {
	if size == 0 {
		return nil
	}

	mask := makeMask(size)
	v := big.NewInt(value)
	if v.Cmp(zero) < 0 {
		v = v.Neg(v)
		v = v.Sub(v, one)
		v = v.Sub(mask, v)
		v = v.And(v, mask)
	}
	return &BVConst{Size: size, mask: mask, value: v}
}

func MakeBVConstFromBigint(value *big.Int, size uint) *BVConst {
	if size == 0 {
		return nil
	}

	mask := makeMask(size)
	v := value
	if v.Cmp(zero) < 0 {
		v = v.Neg(v)
		v = v.Sub(v, one)
		v = v.Sub(mask, v)
		v = v.And(v, mask)
	}
	return &BVConst{Size: size, mask: mask, value: v}
}

func (bv *BVConst) IsNegative() bool {
	return bv.value.Bit(int(bv.Size)-1) == 1
}

func (bv *BVConst) IsZero() bool {
	return bv.value.Cmp(zero) == 0
}

func (bv *BVConst) IsOne() bool {
	return bv.value.Cmp(one) == 0
}

func (bv *BVConst) HasAllBitsSet() bool {
	return bv.value.Cmp(makeMask(bv.Size)) == 0
}

func (bv *BVConst) Copy() *BVConst {
	newVal := big.NewInt(0)
	newMask := big.NewInt(0)

	newVal = newVal.Add(newVal, bv.value)
	newMask = newMask.Add(newMask, bv.mask)
	return &BVConst{Size: bv.Size, mask: newMask, value: newVal}
}

func (bv *BVConst) String() string {
	return fmt.Sprintf("<BV%d 0x%x>", bv.Size, bv.value)
}

func (bv *BVConst) FitInLong() bool {
	maxulong := big.NewInt(2)
	maxulong.Lsh(maxulong, 64)
	maxulong.Sub(maxulong, one)

	return bv.value.Cmp(maxulong) <= 0
}

func (bv *BVConst) AsULong() uint64 {
	// if it does not `FitInLong`, result is undefined
	return bv.value.Uint64()
}

func (bv *BVConst) AsLong() int64 {
	// if it does not `FitInLong`, result is undefined
	if !bv.IsNegative() {
		return bv.value.Int64()
	}
	bvCpy := bv.Copy()
	bvCpy.Not()
	bvCpy.Add(MakeBVConst(1, bv.Size))
	return -int64(bvCpy.AsULong())
}

func (bv *BVConst) Not() {
	bv.value.Not(bv.value)
	bv.value.And(bv.value, bv.mask)
}

func (bv *BVConst) Neg() {
	bv.value.Sub(bv.value, one)
	bv.value.Sub(bv.mask, bv.value)
	bv.value.And(bv.value, bv.mask)
}

func (bv *BVConst) Add(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Add(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVConst) Sub(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Sub(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVConst) Mul(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Mul(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVConst) UDiv(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Div(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVConst) SDiv(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	var c1, c2 *big.Int

	v1 := bv.Copy()
	v2 := o.Copy()
	if v1.IsNegative() {
		v1.Neg()
		c1 = v1.value
		c1 = c1.Neg(c1)
	} else {
		c1 = v1.value
	}
	if v2.IsNegative() {
		v2.Neg()
		c2 = v2.value
		c2 = c2.Neg(c2)
	} else {
		c2 = v2.value
	}

	res := c1.Quo(c1, c2)
	if res.Cmp(zero) < 0 {
		res = res.Neg(res)
		res = res.Sub(res, one)
		res = res.Sub(bv.mask, res)
		res = res.And(res, bv.mask)
	}
	bv.value = res
	return nil
}

func (bv *BVConst) URem(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Rem(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVConst) SRem(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	var c1, c2 *big.Int

	v1 := bv.Copy()
	v2 := o.Copy()
	if v1.IsNegative() {
		v1.Neg()
		c1 = v1.value
		c1 = c1.Neg(c1)
	} else {
		c1 = v1.value
	}
	if v2.IsNegative() {
		v2.Neg()
		c2 = v2.value
		c2 = c2.Neg(c2)
	} else {
		c2 = v2.value
	}

	res := c1.Rem(c1, c2)
	if res.Cmp(zero) < 0 {
		res = res.Neg(res)
		res = res.Sub(res, one)
		res = res.Sub(bv.mask, res)
		res = res.And(res, bv.mask)
	}
	bv.value = res
	return nil
}

func (bv *BVConst) And(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.And(bv.value, o.value)
	return nil
}

func (bv *BVConst) Or(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Or(bv.value, o.value)
	return nil
}

func (bv *BVConst) Xor(o *BVConst) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Xor(bv.value, o.value)
	return nil
}

func (bv *BVConst) AShr(n uint) {
	if n >= bv.Size {
		bv.value = big.NewInt(0)
		return
	}
	if n == 0 {
		return
	}

	isNeg := false
	if bv.IsNegative() {
		isNeg = true
	}

	bv.value = bv.value.Rsh(bv.value, n)
	if isNeg {
		mask := makeMask(bv.Size - n)
		mask = mask.Lsh(mask, n)
		bv.value = bv.value.Or(bv.value, mask)
	}
}

func (bv *BVConst) LShr(n uint) {
	if n >= bv.Size {
		bv.value = big.NewInt(0)
		return
	}
	if n == 0 {
		return
	}

	bv.value = bv.value.Rsh(bv.value, n)
}

func (bv *BVConst) Shl(n uint) {
	if n >= bv.Size {
		bv.value = big.NewInt(0)
		return
	}
	if n == 0 {
		return
	}

	bv.value = bv.value.Lsh(bv.value, n)
}

func (bv *BVConst) Concat(o *BVConst) {
	oCpy := o.Copy()
	oCpy.ZExt(bv.Size)

	bv.ZExt(o.Size)
	bv.Shl(o.Size)
	bv.Or(oCpy)
}

func (bv *BVConst) Truncate(high uint, low uint) error {
	if high < low {
		return fmt.Errorf("high is lower than low")
	}
	if high > bv.Size {
		return fmt.Errorf("high is greater than Size")
	}

	bv.LShr(low)
	bv.Size = high - low + 1
	bv.mask = makeMask(bv.Size)

	bv.value = bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVConst) Slice(high uint, low uint) *BVConst {
	if high < low {
		return nil
	}
	if high > bv.Size {
		return nil
	}

	res := MakeBVConst(0, high-low+1)
	res.value.Or(res.value, bv.value)
	res.value.Rsh(res.value, low)
	res.value.And(res.value, res.mask)
	return res
}

func (bv *BVConst) ZExt(bits uint) {
	bv.Size += bits
	bv.mask = makeMask(bv.Size)
}

func (bv *BVConst) SExt(bits uint) {
	if !bv.IsNegative() {
		bv.ZExt(bits)
		return
	}

	value := bv.value
	newBits := makeMask(bits)
	newBits.Lsh(newBits, uint(bv.Size))
	value = value.Or(newBits, value)
	bv.value = value

	bv.Size += bits
	bv.mask = makeMask(bv.Size)
}

func (bv *BVConst) Eq(o *BVConst) (BoolConst, error) {
	if bv.Size != o.Size {
		return BoolTrue(), fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	if bv.value == o.value {
		return BoolTrue(), nil
	}
	return BoolFalse(), nil
}

func (bv *BVConst) NEq(o *BVConst) (BoolConst, error) {
	if bv.Size != o.Size {
		return BoolTrue(), fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	if bv.value != o.value {
		return BoolTrue(), nil
	}
	return BoolFalse(), nil
}

func (bv *BVConst) UGt(o *BVConst) (BoolConst, error) {
	if bv.Size != o.Size {
		return BoolTrue(), fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	if bv.value.CmpAbs(o.value) > 0 {
		return BoolTrue(), nil
	}
	return BoolFalse(), nil
}

func (bv *BVConst) UGe(o *BVConst) (BoolConst, error) {
	v, err := bv.Eq(o)
	if err != nil || v.Value {
		return BoolTrue(), err
	}
	return bv.UGt(o)
}

func (bv *BVConst) Ult(o *BVConst) (BoolConst, error) {
	v, err := bv.UGe(o)
	return v.Not(), err
}

func (bv *BVConst) Ule(o *BVConst) (BoolConst, error) {
	v, err := bv.UGt(o)
	return v.Not(), err
}

func (bv *BVConst) SGt(o *BVConst) (BoolConst, error) {
	if bv.Size != o.Size {
		return BoolTrue(), fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	if bv.IsNegative() && !o.IsNegative() {
		return BoolFalse(), nil
	}
	if !bv.IsNegative() && o.IsNegative() {
		return BoolTrue(), nil
	}
	if bv.IsNegative() && o.IsNegative() {
		if bv.value.CmpAbs(o.value) > 0 {
			return BoolTrue(), nil
		}
		return BoolFalse(), nil
	}

	if bv.value.CmpAbs(o.value) > 0 {
		return BoolTrue(), nil
	}
	return BoolFalse(), nil
}

func (bv *BVConst) SGe(o *BVConst) (BoolConst, error) {
	v, err := bv.Eq(o)
	if err != nil || v.Value {
		return BoolTrue(), err
	}
	return bv.SGt(o)
}

func (bv *BVConst) SLt(o *BVConst) (BoolConst, error) {
	v, err := bv.SGe(o)
	return v.Not(), err
}

func (bv *BVConst) SLe(o *BVConst) (BoolConst, error) {
	v, err := bv.SGt(o)
	return v.Not(), err
}
