package gosmt

type BoolV struct {
	Value bool
}

func (b BoolV) String() string {
	if b.Value {
		return "T"
	}
	return "F"
}

func BoolTrue() BoolV {
	return BoolV{true}
}

func BoolFalse() BoolV {
	return BoolV{false}
}

func (b BoolV) Not() BoolV {
	return BoolV{!b.Value}
}

func (b BoolV) And(o BoolV) BoolV {
	return BoolV{b.Value && o.Value}
}

func (b BoolV) Or(o BoolV) BoolV {
	return BoolV{b.Value || o.Value}
}

func (b BoolV) Xor(o BoolV) BoolV {
	return BoolV{b.Value != o.Value}
}
