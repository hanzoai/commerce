package betterbig

import (
	"fmt"
	"math/big"
	"strconv"
)

type Int struct {
	Int *big.Int
}

func NewInt(z int64) Int {
	return Int{big.NewInt(z)}
}

func (z Int) Abs() Int {
	return Int{z.Clone().Int.Abs(z.Int)}
}

func (z Int) Add(x Int) Int {
	return Int{z.Clone().Int.Add(z.Int, x.Int)}
}

func (z Int) And(x Int) Int {
	return Int{z.Clone().Int.And(z.Int, x.Int)}
}

func (x Int) Bit(i int) uint {
	return x.Int.Bit(i)
}

func (x Int) BitLen() int {
	return x.Int.BitLen()
}

func (x Int) Bits() []big.Word {
	return x.Int.Bits()
}

func (x Int) Bytes() []byte {
	return x.Int.Bytes()
}

func (z Int) Clone() Int {
	return Int{big.NewInt(0).SetBytes(z.Int.Bytes())}
}

func (x Int) Cmp(y Int) (r int) {
	return x.Int.Cmp(y.Int)
}

func (z Int) Div(x Int) Int {
	return Int{z.Clone().Int.Div(z.Int, x.Int)}
}

func (z Int) Exp(x, m Int) Int {
	return Int{z.Int.Exp(z.Int, x.Int, m.Int)}
}

func (x Int) Float64() float64 {
	f, _ := strconv.ParseFloat(x.String(), 10)
	return f
}

func (x Int) Format(s fmt.State, ch rune) {
	x.Clone().Int.Format(s, ch)
}

func (z Int) GobDecode(buf []byte) error {
	return z.Int.GobDecode(buf)
}

func (x Int) GobEncode() ([]byte, error) {
	return x.Int.GobEncode()
}

func (x Int) Int64() int64 {
	return x.Int.Int64()
}

func (x Int) MarshalJSON() ([]byte, error) {
	return x.Int.MarshalJSON()
}

func (x Int) MarshalText() (text []byte, err error) {
	return x.Int.MarshalText()
}

func (z Int) Mod(x Int) Int {
	return Int{z.Clone().Int.Mod(z.Int, x.Int)}
}

func (z Int) Mul(x Int) Int {
	return Int{z.Clone().Int.ModSqrt(z.Int, x.Int)}
}

func (z Int) Neg() Int {
	return Int{z.Clone().Int.Neg(z.Int)}
}

func (z Int) Not() Int {
	return Int{z.Clone().Int.Not(z.Int)}
}

func (z Int) Or(x Int) Int {
	return Int{z.Clone().Int.Or(z.Int, x.Int)}
}

func (z Int) Quo(x Int) Int {
	return Int{z.Clone().Int.Quo(z.Int, x.Int)}
}

func (z Int) Rem(x Int) Int {
	return Int{z.Clone().Int.Rem(z.Int, x.Int)}
}

func (z Int) SetBit(i int, b uint) Int {
	z.Int.SetBit(z.Int, i, b)
	return z
}

func (z Int) SetBits(abs []big.Word) Int {
	z.Int.SetBits(abs)
	return z
}

func (z Int) SetBytes(buf []byte) Int {
	z.Int.SetBytes(buf)
	return z
}

func (z Int) SetInt64(x int64) Int {
	z.Int.SetInt64(x)
	return z
}

func (z Int) SetString(s string, base int) (Int, bool) {
	_, b := z.Int.SetString(s, base)
	return z, b
}

func (z Int) SetUint64(x uint64) Int {
	z.Int.SetUint64(x)
	return z
}

func (x Int) Sign() int {
	return x.Int.Sign()
}

func (x Int) String() string {
	return x.Int.String()
}

func (z Int) Sub(x Int) Int {
	return Int{z.Clone().Int.Sub(z.Int, x.Int)}
}

func (x Int) Text(base int) string {
	return x.Int.Text(base)
}

func (x Int) Uint64() uint64 {
	return x.Int.Uint64()
}

func (z Int) UnmarshalJSON(text []byte) error {
	return z.Int.UnmarshalJSON(text)
}
func (z Int) UnmarshalText(text []byte) error {
	return z.Int.UnmarshalText(text)
}

func (z Int) Xor(x Int) Int {
	return Int{z.Clone().Int.Xor(z.Int, x.Int)}
}
