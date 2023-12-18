package gosmt

import (
	"fmt"
	"math/big"
)

var zero = big.NewInt(0)
var one = big.NewInt(1)

type BV struct {
	Size  int
	mask  *big.Int
	value *big.Int
}

func makeMask(size int) *big.Int {
	bytes := make([]byte, size/8)
	for i := 0; i < size/8; i++ {
		bytes[i] = 0xff
	}
	v := big.NewInt(0)
	v.SetBytes(bytes)
	for i := size / 8; i < size/8+size%8; i++ {
		v.SetBit(v, i, 1)
	}
	return v
}

func MakeBV(value int64, size int) *BV {
	mask := makeMask(size)

	v := big.NewInt(value)
	if v.Cmp(zero) < 0 {
		v = v.Neg(v)
		v = v.Sub(v, one)
		v = v.Sub(mask, v)
		v = v.And(v, mask)
	}
	return &BV{Size: size, mask: mask, value: v}
}

func (bv *BV) Copy() *BV {
	newVal := *bv.value
	return &BV{Size: bv.Size, mask: bv.mask, value: &newVal}
}

func (bv *BV) String() string {
	return fmt.Sprintf("<BV%d 0x%x>", bv.Size, bv.value)
}

func (bv *BV) FitInLong() bool {
	maxulong := big.NewInt(2)
	maxulong.Lsh(maxulong, 64)
	maxulong.Sub(maxulong, one)

	return bv.value.Cmp(maxulong) <= 0
}

func (bv *BV) AsULong() uint64 {
	// if it does not `FitInLong`, result is undefined
	return bv.value.Uint64()
}

func (bv *BV) AsLong() int64 {
	// if it does not `FitInLong`, result is undefined
	if bv.value.Bit(bv.Size-1) == 0 {
		return bv.value.Int64()
	}
	bvCpy := bv.Copy()
	bvCpy.Not()
	bvCpy.Add(MakeBV(1, bv.Size))
	return -int64(bvCpy.AsULong())
}

func (bv *BV) Not() {
	bv.value = bv.value.Not(bv.value)
	bv.value.And(bv.value, bv.mask)
}

func (bv *BV) Add(o *BV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Add(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BV) Sub(o *BV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Sub(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BV) Mul(o *BV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Mul(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BV) Div(o *BV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Div(bv.value, o.value)
	bv.value.And(bv.value, bv.mask)
	return nil
}

func (bv *BV) And(o *BV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.And(bv.value, o.value)
	return nil
}

func (bv *BV) Or(o *BV) error {
	if bv.Size != o.Size {
		return fmt.Errorf("different sizes %d and %d", bv.Size, o.Size)
	}

	bv.value = bv.value.Or(bv.value, o.value)
	return nil
}

func (bv *BV) ZExt(bits int) {
	bv.Size += bits
	bv.mask = makeMask(bv.Size)
}

func (bv *BV) SExt(bits int) {
	if bv.value.Bit(bv.Size-1) == 0 {
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
