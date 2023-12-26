package gosmt

type BoolConst struct {
	Value bool
}

func (b BoolConst) String() string {
	if b.Value {
		return "T"
	}
	return "F"
}

func BoolTrue() BoolConst {
	return BoolConst{true}
}

func BoolFalse() BoolConst {
	return BoolConst{false}
}

func (b BoolConst) Not() BoolConst {
	return BoolConst{!b.Value}
}

func (b BoolConst) And(o BoolConst) BoolConst {
	return BoolConst{b.Value && o.Value}
}

func (b BoolConst) Or(o BoolConst) BoolConst {
	return BoolConst{b.Value || o.Value}
}

func (b BoolConst) Xor(o BoolConst) BoolConst {
	return BoolConst{b.Value != o.Value}
}
