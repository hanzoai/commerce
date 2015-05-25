package bitstring

type Segment int64

type BitString struct {
	segments []Segment
	length   int
	mask     Segment
}

var one = Segment(1)
var zero = Segment(0)
var full = ^Segment(0)

func segments(bs *BitString) uint {
	return uint(bs.length / 64)
}

func New(length int) *BitString {
	lastSegmentBitCount := uint(length % 64)
	mask := Segment(full << lastSegmentBitCount)
	return &BitString{make([]Segment, length/64), length, mask}
}

func (b *BitString) SetBit(idx uint) *BitString {
	segI := b.GetSegment(idx)

	if segI > uint(len(b.segments)) {
		return b
	}

	segCount := segments(b)
	segs := make([]Segment, segCount)
	for i := uint(0); i < segCount; i++ {
		if i == segI {
			addrI := b.GetAddress(idx)

			seg := b.segments[segI]
			seg |= (1 << addrI)
			segs[i] = seg
		} else {
			segs[i] = b.segments[segI]
		}
	}

	return &BitString{segs, b.length, b.mask}
}

func (b *BitString) ClearBit(idx uint) *BitString {
	segI := b.GetSegment(idx)

	if segI > uint(len(b.segments)) {
		return b
	}

	segCount := segments(b)
	segs := make([]Segment, segCount)
	for i := uint(0); i < segCount; i++ {
		if i == segI {
			addrI := b.GetAddress(idx)

			seg := b.segments[segI]
			seg &= ^(1 << addrI)
			segs[i] = seg
		} else {
			segs[i] = b.segments[segI]
		}
	}

	return &BitString{segs, b.length, b.mask}
}

func (b *BitString) GetBit(idx uint) bool {
	segI := b.GetSegment(idx)
	if segI > uint(len(b.segments)) {
		return false
	}

	addrI := b.GetAddress(idx)
	seg := b.segments[segI]

	return seg&(1<<addrI) > 0
}

func (b *BitString) Equal(b2 *BitString) bool {
	segCount := segments(b)
	for i := uint(0); i < segCount; i++ {
		if b.segments[i] != b2.segments[i] {
			return false
		}
	}
	return true
}

func (b *BitString) GetSegment(idx uint) uint {
	return idx / 64
}

func (b *BitString) GetAddress(idx uint) uint {
	return idx % 64
}
