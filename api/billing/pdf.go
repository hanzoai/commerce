package billing

// The invoice's PDF writer: Letter pages in Hanzo's type (Zen, embedded) with
// the issuing brand's mark drawn as vector paths. No library: a PDF this small
// is a catalog, a page tree, two fonts and a content stream per page, and the
// writer stays a pure function of its input, so the same invoice renders the
// same bytes every time.

import (
	"bytes"
	"compress/zlib"
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Zen Regular and SemiBold (the Hanzo type family, @hanzo/font, SIL OFL 1.1 in
// fonts/OFL.txt), cut to the Windows-1252 repertoire the pages encode text in.
//
//go:embed fonts/Zen-Regular.ttf
var zenRegular []byte

//go:embed fonts/Zen-SemiBold.ttf
var zenSemiBold []byte

type weight int

const (
	regular weight = iota
	semibold
)

// face is one embedded TrueType font and what a reader needs to place it: the
// advance of every Windows-1252 code and the descriptor's metrics, in 1/1000 em.
type face struct {
	name                       string
	file                       []byte
	width                      [256]int
	ascent, descent, capHeight int
	bbox                       [4]int
	stemV                      int
}

var faces = [...]*face{
	regular:  mustFace("HZNAAA+Zen-Regular", zenRegular, 80),
	semibold: mustFace("HZNAAB+Zen-SemiBold", zenSemiBold, 120),
}

// mustFace reads an embedded font. The files are this package's own, so one
// that does not parse is a build defect and stops the binary at start, before
// any invoice could render without its type.
func mustFace(name string, file []byte, stemV int) *face {
	f, err := sfnt.Parse(file)
	if err != nil {
		panic(fmt.Sprintf("invoice font %s: %v", name, err))
	}
	var buf sfnt.Buffer
	upem := fixed.I(int(f.UnitsPerEm()))
	per := func(v fixed.Int26_6) int { return int(math.Round(float64(v) / float64(upem) * 1000)) }
	fc := &face{name: name, file: file, stemV: stemV}
	for code := 32; code < 256; code++ {
		r := decode1252(byte(code))
		if r == 0 {
			continue
		}
		gi, err := f.GlyphIndex(&buf, r)
		if err != nil || gi == 0 {
			continue
		}
		adv, err := f.GlyphAdvance(&buf, gi, upem, font.HintingNone)
		if err != nil {
			panic(fmt.Sprintf("invoice font %s: advance of %q: %v", name, r, err))
		}
		fc.width[code] = per(adv)
	}
	m, err := f.Metrics(&buf, upem, font.HintingNone)
	if err != nil {
		panic(fmt.Sprintf("invoice font %s: metrics: %v", name, err))
	}
	b, err := f.Bounds(&buf, upem, font.HintingNone)
	if err != nil {
		panic(fmt.Sprintf("invoice font %s: bounds: %v", name, err))
	}
	fc.ascent, fc.descent, fc.capHeight = per(m.Ascent), -per(m.Descent), per(m.CapHeight)
	// sfnt measures y downward; a PDF's y runs up.
	fc.bbox = [4]int{per(b.Min.X), -per(b.Max.Y), per(b.Max.X), -per(b.Min.Y)}
	return fc
}

// ── Windows-1252 ─────────────────────────────────────────────────────────────

// cp1252 is what 0x80–0x9F stand for in Windows-1252; the rest of 0x20–0xFF is
// Latin-1, which is Unicode's first 256 code points.
var cp1252 = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

func decode1252(b byte) rune {
	switch {
	case b >= 0x80 && b < 0xA0:
		return cp1252[b-0x80]
	case b >= 0x20 && b != 0x7F:
		return rune(b)
	}
	return 0
}

// encode1252 spells s in the pages' encoding. A control character is dropped; a
// character the encoding has no code for reads as "?", so a name in another
// script still shows that something was there.
func encode1252(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7F || (r >= 0x80 && r < 0xA0):
			continue
		case r < 0x100:
			out = append(out, byte(r))
		default:
			c := byte('?')
			for i, x := range cp1252 {
				if x == r {
					c = byte(0x80 + i)
					break
				}
			}
			out = append(out, c)
		}
	}
	return out
}

// measure is the width of s set in w at size with track points after each
// glyph but the last.
func measure(w weight, size, track float64, s string) float64 {
	enc := encode1252(s)
	units := 0
	for _, c := range enc {
		units += faces[w].width[c]
	}
	width := float64(units) * size / 1000
	if len(enc) > 1 {
		width += track * float64(len(enc)-1)
	}
	return width
}

// fit shortens s with an ellipsis until it is no wider than max.
func fit(w weight, size float64, s string, max float64) string {
	if measure(w, size, 0, s) <= max {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && measure(w, size, 0, string(r)+"…") > max {
		r = r[:len(r)-1]
	}
	return strings.TrimRight(string(r), " ") + "…"
}

// ── Drawing ──────────────────────────────────────────────────────────────────

// num prints a coordinate with at most two decimals and no trailing zeros, so
// the stream is short and the same value always prints the same.
func num(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}

// canvas is one page's content stream. Coordinates are points from the
// bottom-left corner; a gray is 0 (black) to 1 (white).
type canvas struct{ b strings.Builder }

func (c *canvas) op(format string, a ...any) { fmt.Fprintf(&c.b, format+"\n", a...) }

// text sets s with its left edge at x on baseline y.
func (c *canvas) text(w weight, size, x, y, gray, track float64, s string) {
	c.op("BT /F%d %s Tf %s g %s Tc 1 0 0 1 %s %s Tm (%s) Tj ET",
		int(w)+1, num(size), num(gray), num(track), num(x), num(y), literal(encode1252(s)))
}

// textRight sets s with its right edge at x.
func (c *canvas) textRight(w weight, size, x, y, gray, track float64, s string) {
	c.text(w, size, x-measure(w, size, track, s), y, gray, track, s)
}

// rule is a horizontal line from x1 to x2 at y.
func (c *canvas) rule(x1, x2, y, width, gray float64) {
	c.op("%s G %s w %s %s m %s %s l S", num(gray), num(width), num(x1), num(y), num(x2), num(y))
}

// capsule traces a rectangle whose short ends are half circles.
func (c *canvas) capsule(x, y, w, h float64) {
	r := h / 2
	k := r * 0.5523
	c.op("%s %s m %s %s l", num(x+r), num(y), num(x+w-r), num(y))
	c.op("%s %s %s %s %s %s c", num(x+w-r+k), num(y), num(x+w), num(y+r-k), num(x+w), num(y+r))
	c.op("%s %s %s %s %s %s c", num(x+w), num(y+r+k), num(x+w-r+k), num(y+h), num(x+w-r), num(y+h))
	c.op("%s %s l", num(x+r), num(y+h))
	c.op("%s %s %s %s %s %s c", num(x+r-k), num(y+h), num(x), num(y+r+k), num(x), num(y+r))
	c.op("%s %s %s %s %s %s c h", num(x), num(y+r-k), num(x+r-k), num(y), num(x+r), num(y))
}

// pill draws a status label in a capsule whose right edge is at x and whose
// bottom is at y: filled black with white type when solid, else an outline in
// the label's gray.
func (c *canvas) pill(label string, x, y, gray float64, solid bool) {
	const size, track, padX, h = 7.5, 0.9, 8.0, 16.0
	w := measure(semibold, size, track, label) + 2*padX
	c.capsule(x-w, y, w, h)
	ink := gray
	if solid {
		c.op("%s g f", num(gray))
		ink = 1
	} else {
		c.op("%s G 0.75 w S", num(gray))
	}
	c.text(semibold, size, x-w+padX, y+5.1, ink, track, label)
}

// mark draws the brand's mark in a size × size square whose bottom-left corner
// is at x, y.
func (c *canvas) mark(m *mark, x, y, size float64) {
	s := size / m.view
	// The mark is drawn in its SVG space: y runs down from the square's top.
	c.op("q %s 0 0 %s %s %s cm", num(s), num(-s), num(x), num(y+size))
	c.b.WriteString(m.ops)
	c.op("Q")
}

// literal is a PDF string literal of already-encoded bytes. The delimiters and
// the escape are escaped, and every byte outside printable ASCII is written as
// an octal escape, so the stream stays plain text.
func literal(enc []byte) string {
	var b strings.Builder
	for _, c := range enc {
		switch {
		case c == '\\' || c == '(' || c == ')':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c < 0x20 || c > 0x7E:
			fmt.Fprintf(&b, "\\%03o", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ── Marks ────────────────────────────────────────────────────────────────────

// mark is a brand's mark as PDF path operators in its own square SVG space.
type mark struct {
	view float64
	ops  string
}

// pathOps turns SVG path data made of absolute M, L, H, V and Z commands (all
// the Hanzo and Lux marks use) into PDF path operators.
func pathOps(d string) string {
	var b strings.Builder
	var x, y float64
	cmd := byte(0)
	toks := strings.FieldsFunc(strings.NewReplacer("M", " M ", "L", " L ", "H", " H ", "V", " V ", "Z", " Z ", ",", " ").Replace(d),
		func(r rune) bool { return r == ' ' })
	next := func(i *int) float64 {
		*i++
		v, err := strconv.ParseFloat(toks[*i], 64)
		if err != nil {
			panic(fmt.Sprintf("mark path %q: %v", d, err))
		}
		return v
	}
	for i := 0; i < len(toks); i++ {
		switch t := toks[i]; t {
		case "M", "L", "H", "V", "Z":
			cmd = t[0]
			if cmd == 'Z' {
				b.WriteString("h\n")
				continue
			}
		default:
			i-- // a repeated coordinate pair continues the last command
		}
		switch cmd {
		case 'M':
			x, y = next(&i), next(&i)
			fmt.Fprintf(&b, "%s %s m\n", num(x), num(y))
			cmd = 'L'
		case 'L':
			x, y = next(&i), next(&i)
			fmt.Fprintf(&b, "%s %s l\n", num(x), num(y))
		case 'H':
			x = next(&i)
			fmt.Fprintf(&b, "%s %s l\n", num(x), num(y))
		case 'V':
			y = next(&i)
			fmt.Fprintf(&b, "%s %s l\n", num(x), num(y))
		}
	}
	return b.String()
}

// circleOps traces a circle as four cubic curves.
func circleOps(cx, cy, r float64) string {
	k := r * 0.5523
	return fmt.Sprintf("%s %s m\n%s %s %s %s %s %s c\n%s %s %s %s %s %s c\n%s %s %s %s %s %s c\n%s %s %s %s %s %s c h\n",
		num(cx+r), num(cy),
		num(cx+r), num(cy+k), num(cx+k), num(cy+r), num(cx), num(cy+r),
		num(cx-k), num(cy+r), num(cx-r), num(cy+k), num(cx-r), num(cy),
		num(cx-r), num(cy-k), num(cx-k), num(cy-r), num(cx), num(cy-r),
		num(cx+k), num(cy-r), num(cx+r), num(cy-k), num(cx+r), num(cy))
}

// The Hanzo mark: @hanzo/logo's MARK_BLOCKS in black and its two MARK_SHADE
// slivers in their #DDDDDD (src/logos.ts), on the 67 square.
var hanzoMark = func() *mark {
	var b strings.Builder
	b.WriteString("0 g\n")
	for _, d := range []string{
		"M22.21 67V44.6369H0V67H22.21Z",
		"M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z",
		"M22.21 0H0V22.3184H22.21V0Z",
		"M66.7198 0H44.5098V22.3184H66.7198V0Z",
		"M66.7198 67V44.6369H44.5098V67H66.7198Z",
	} {
		b.WriteString(pathOps(d))
		b.WriteString("f\n")
	}
	b.WriteString("0.867 g\n")
	for _, d := range []string{
		"M0 44.6369L22.21 46.8285V44.6369H0Z",
		"M66.6753 22.3185L44.5098 20.0822V22.3185H66.6753Z",
	} {
		b.WriteString(pathOps(d))
		b.WriteString("f\n")
	}
	return &mark{view: 67, ops: b.String()}
}()

// The Lux mark: @hanzo/brand's LUX_MARK triangle (src/marks.ts) on the 100 square.
var luxMark = &mark{view: 100, ops: "0 g\n" + pathOps("M50 85 L15 25 L85 25 Z") + "f\n"}

// The Zoo mark: @zooai/logo's logo-mono.svg, three rings and the ring that holds
// them, stroked and clipped to the outer circle, on the 1024 square.
var zooMark = &mark{view: 1024, ops: "0 G\n" + circleOps(508, 510, 283) + "W n\n" +
	"33 w\n" + circleOps(513, 369, 234) + "S\n" + circleOps(365, 595, 234) + "S\n" + circleOps(643, 595, 234) + "S\n" +
	"36 w\n" + circleOps(508, 510, 265) + "S\n"}

// ── File ─────────────────────────────────────────────────────────────────────

// writePDF assembles the pages' content streams into a PDF file titled title.
func writePDF(title string, pages []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	var offsets []int
	obj := func(body string) int {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
		return len(offsets)
	}
	stream := func(dict string, data []byte) int {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n<< %s /Length %d >>\nstream\n", len(offsets), dict, len(data))
		buf.Write(data)
		buf.WriteString("\nendstream\nendobj\n")
		return len(offsets)
	}

	// 1 catalog, 2 page tree, 3 info; the fonts; then each page and its stream.
	first := 4 + 3*len(faces)
	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", first+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	obj(fmt.Sprintf("<< /Title (%s) >>", literal(encode1252(title))))

	fontRes := make([]string, len(faces))
	for i, f := range faces {
		widths := make([]string, 0, 224)
		for code := 32; code < 256; code++ {
			widths = append(widths, strconv.Itoa(f.width[code]))
		}
		id := obj(fmt.Sprintf("<< /Type /Font /Subtype /TrueType /BaseFont /%s /FirstChar 32 /LastChar 255 /Widths [%s] /FontDescriptor %d 0 R /Encoding /WinAnsiEncoding >>",
			f.name, strings.Join(widths, " "), len(offsets)+2))
		obj(fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags 32 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV %d /FontFile2 %d 0 R >>",
			f.name, f.bbox[0], f.bbox[1], f.bbox[2], f.bbox[3], f.ascent, f.descent, f.capHeight, f.stemV, len(offsets)+2))
		stream(fmt.Sprintf("/Filter /FlateDecode /Length1 %d", len(f.file)), deflate(f.file))
		fontRes[i] = fmt.Sprintf("/F%d %d 0 R", i+1, id)
	}

	for i, content := range pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources << /Font << %s >> >> /Contents %d 0 R >>",
			num(pageW), num(pageH), strings.Join(fontRes, " "), first+2*i+1))
		stream("", []byte(content))
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R /Info 3 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return buf.Bytes()
}

// deflate compresses an embedded font file. The level is fixed, so the output
// is the same for the same input.
func deflate(b []byte) []byte {
	var out bytes.Buffer
	w, _ := zlib.NewWriterLevel(&out, zlib.BestCompression)
	w.Write(b)
	w.Close()
	return out.Bytes()
}
