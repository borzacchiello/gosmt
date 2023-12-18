package gosmt

import (
	"fmt"
	"math/big"
)

var zero = big.NewInt(0)
var one = big.NewInt(1)

type BVV struct {
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

func MakeBV(value int64, size uint) *BVV {
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
	return &BVV{Size: size, mask: mask, value: v}
}

func (bv *BVV) IsNegative() bool {
	return bv.value.Bit(int(bv.Size)-1) == 1
}

func (bv *BVV) Copy() *BVV {
	newVal := big.NewInt(0)
	newMask := big.NewInt(0)

	newVal = newVal.Add(newVal, bv.value)
	newMask = newMask.Add(newMask, bv.mask)
	return &BVV{Size: bv.Size, mask: newMask, value: newVal}
}

func (bv *BVV) String() string {
	return fmt.Sprintf("<BV%d 0x%x>", bv.Size, bv.value)
}

func (bv *BVV) FitInLong() bool {
	maxulong := big.NewInt(2)
	maxulong.Lsh(maxulong, 64)
	maxulong.Sub(maxulong, one)

	return bv.value.Cmp(maxulong) <= 0
}

func (bv *BVV) AsULong() uint64 {
	// if it does not `FitInLong`, result is undefined
	return bv.value.Uint64()
}

func (bv *BVV) AsLong() int64 {
	// if it does not `FitInLong`, result is undefined
	if !bv.IsNegative() {
		return bv.value.Int64()
	}
	bvCpy := bv.Copy()
	bvCpy.Not()
	bvCpy.Add(MakeBV(1, bv.Size))
	return -int64(bvCpy.AsULong())
}

func (bv *BVV) Not() {
	bv.value.Not(bv.value)
	bv.value.And(bv.value, bv.mask)
}

func (bv *BVV) Neg() {
	bv.value.Sub(bv.value, one)
	bv.value.Sub(bv.mask, bv.value)
	bv.value.And(bv.value, bv.mask)
}

func (bv *BVV) Add(o *BVV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Add(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVV) Sub(o *BVV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Sub(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVV) Mul(o *BVV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Mul(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVV) Div(o *BVV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Div(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BVV) And(o *BVV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.And(bv.value, o.value)
	return nil
}

func (bv *BVV) Or(o *BVV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Or(bv.value, o.value)
	return nil
}

func (bv *BVV) AShr(n uint) {
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

func (bv *BVV) LShr(n uint) {
	if n >= bv.Size {
		bv.value = big.NewInt(0)
		return
	}
	if n == 0 {
		return
	}

	bv.value = bv.value.Rsh(bv.value, n)
}

func (bv *BVV) Shl(n uint) {
	if n >= bv.Size {
		bv.value = big.NewInt(0)
		return
	}
	if n == 0 {
		return
	}

	bv.value = bv.value.Lsh(bv.value, n)
}

func (bv *BVV) Concat(o *BVV) {
	oCpy := o.Copy()
	oCpy.ZExt(bv.Size)

	bv.ZExt(o.Size)
	bv.Shl(o.Size)
	bv.Or(oCpy)
}

func (bv *BVV) Truncate(high uint, low uint) error {
	if low < 0 || high < 0 {
		return fmt.Errorf("high or low cannot be less than zero")
	}
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

func (bv *BVV) Slice(high uint, low uint) *BVV {
	if low < 0 || high < 0 {
		return nil
	}
	if high < low {
		return nil
	}
	if high > bv.Size {
		return nil
	}

	res := MakeBV(0, high-low+1)
	res.value.Or(res.value, bv.value)
	res.value.Rsh(res.value, low)
	res.value.And(res.value, res.mask)
	return res
}

func (bv *BVV) ZExt(bits uint) {
	bv.Size += bits
	bv.mask = makeMask(bv.Size)
}

func (bv *BVV) SExt(bits uint) {
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
