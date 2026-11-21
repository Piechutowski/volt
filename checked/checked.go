package checked

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func Add[T Integer](augend T, addend T) (T, bool) {
	sum := augend + addend
	return sum, (sum > augend) == (addend > 0)
}

func Subtract[T Integer](minuend T, subtrahend T) (T, bool) {
	difference := minuend - subtrahend
	return difference, (difference < minuend) == (subtrahend > 0)
}

func Multiply[T Integer](multiplicand T, multiplier T) (T, bool) {
	product := multiplicand * multiplier
	if multiplicand == 0 {
		return product, true
	}
	if multiplier == 0 {
		return product, true
	}
	if isOverflowNegated(multiplier, multiplicand) {
		return product, false
	}
	return product, product/multiplier == multiplicand
}

func Convert[To Integer, From Integer](value From) (To, bool) {
	converted := To(value)
	if From(converted) != value {
		return converted, false
	}
	return converted, (converted < 0) == (value < 0)
}

func isOverflowNegated[T Integer](negator T, value T) bool {
	return isNegativeOne(negator) && isMinimum(value)
}

func isNegativeOne[T Integer](value T) bool {
	return value < 0 && value+1 == 0
}

func isMinimum[T Integer](value T) bool {
	return value < 0 && -value == value
}
