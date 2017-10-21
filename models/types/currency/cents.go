package currency

import (
	// "fmt"
	// . "math/big"
	"strconv"
)

type Cents int64

// type Cents struct {
// 	*Int
// }

// func NewCents(z int64) Cents {
// 	return Cents{NewInt(z)}
// }

// func (z Cents) Abs() Cents {
// 	return Cents{z.Clone().Int.Abs(z.Int)}
// }

// func (z Cents) Add(x *Int) Cents {
// 	return Cents{z.Clone().Int.Add(z.Int, x)}
// }

// func (z Cents) And(x *Int) Cents {
// 	return Cents{z.Clone().Int.And(z.Int, x)}
// }

// func (x Cents) Bit(i int) uint {
// 	return x.Int.Bit(i)
// }

// func (x Cents) BitLen() int {
// 	return x.Int.BitLen()
// }

// func (x Cents) Bits() []Word {
// 	return x.Int.Bits()
// }

// func (x Cents) Bytes() []byte {
// 	return x.Int.Bytes()
// }

// func (z Cents) Clone() Cents {
// 	return Cents{NewInt(0).SetBytes(z.Int.Bytes())}
// }

// func (x Cents) Cmp(y Cents) (r int) {
// 	return x.Int.Cmp(y.Int)
// }

// func (z Cents) Div(x *Int) Cents {
// 	return Cents{z.Clone().Int.Div(z.Int, x)}
// }

// func (z Cents) Exp(x, m *Int) Cents {
// 	return Cents{z.Int.Exp(z.Int, x, m)}
// }

// func (x Cents) Format(s fmt.State, ch rune) {
// 	x.Clone().Int.Format(s, ch)
// }

// func (z Cents) GobDecode(buf []byte) error {
// 	return z.Int.GobDecode(buf)
// }

// func (x Cents) GobEncode() ([]byte, error) {
// 	return x.Int.GobEncode()
// }

// func (x Cents) Int64() int64 {
// 	return x.Int.Int64()
// }

// func (x Cents) IsInt64() bool {
// 	return x.Int.IsInt64()
// }

// func (x Cents) IsUint64() bool {
// 	return x.Int.IsUint64()
// }

// func (x Cents) MarshalJSON() ([]byte, error) {
// 	return x.Int.MarshalJSON()
// }

// func (x Cents) MarshalText() (text []byte, err error) {
// 	return x.Int.MarshalText()
// }

// func (z Cents) Mod(x *Int) Cents {
// 	return Cents{z.Clone().Int.Mod(z.Int, x)}
// }

// func (z Cents) Mul(x *Int) Cents {
// 	return Cents{z.Clone().Int.ModSqrt(z.Int, x)}
// }

// func (z Cents) Neg() Cents {
// 	return Cents{z.Clone().Int.Neg(z.Int)}
// }

// func (z Cents) Not() Cents {
// 	return Cents{z.Clone().Int.Not(z.Int)}
// }

// func (z Cents) Or(x *Int) Cents {
// 	return Cents{z.Clone().Int.Or(z.Int, x)}
// }

// func (z Cents) Quo(x *Int) Cents {
// 	return Cents{z.Clone().Int.Quo(z.Int, x)}
// }

// func (z Cents) Rem(x *Int) Cents {
// 	return Cents{z.Clone().Int.Rem(z.Int, x)}
// }

// func (z Cents) Scan(s fmt.ScanState, ch rune) error {
// 	return z.Int.Scan(s, ch)
// }

// func (z Cents) Set(x *Int) Cents {
// 	z.Int.Set(x)
// 	return z
// }

// func (z Cents) SetBit(i int, b uint) Cents {
// 	z.Int.SetBit(z.Int, i, b)
// 	return z
// }

// func (z Cents) SetBits(abs []Word) Cents {
// 	z.Int.SetBits(abs)
// 	return z
// }

// func (z Cents) SetBytes(buf []byte) Cents {
// 	return z.SetBytes(buf)
// }

// func (z Cents) SetInt64(x int64) Cents {
// 	return z.SetInt64(x)
// }

// func (z Cents) SetString(s string, base int) (Cents, bool) {
// 	return z.SetString(s, base)
// }

// func (z Cents) SetUint64(x uint64) Cents {
// 	return z.SetUint64(x)
// }

// func (x Cents) Sign() int {
// 	return x.Sign()
// }

// func (z Cents) Sqrt(x Cents) Cents {
// 	return z.Sqrt(x)
// }

// func (x Cents) String() string {
// 	return x.String()
// }

// func (z Cents) Sub(x, y Cents) Cents {
// 	return z.Sub(x, y)
// }

// func (x Cents) Text(base int) string {
// 	return x.Text(base)
// }

// func (x Cents) Uint64() uint64 {
// 	return x.Uint64()
// }

// func (z Cents) UnmarshalJSON(text []byte) error {
// 	return z.UnmarshalJSON(text)
// }
// func (z Cents) UnmarshalText(text []byte) error {
// 	return z.UnmarshalText(text)
// }

// func (z Cents) Xor(x, y Cents) Cents {
// 	return z.Xor(x, y)
// }

func CentsFromString(s string) Cents {
	f, _ := strconv.ParseFloat(s, 64)
	return Cents(int64(f * 100))
}
