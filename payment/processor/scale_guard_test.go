package processor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// A money amount's scale comes from its currency and from nowhere else. There
// are exactly three sanctioned conversions — currency.Type.Parse (text to minor
// units), Type.Amount / money.FromMinor (minor units to a money.Amount), and
// Type.ToStringNoSymbol / Amount.MajorString (back to text) — and each reads
// Type.Decimals() to learn how many fractional digits the currency actually has.
//
// This test fails the build on the shortcut those exist to replace: a literal
// 100 multiplied or divided against a money value, or a "%.2f" that renders one.
// The literal is wrong twice over. It assumes two decimal places, so a
// zero-decimal currency (JPY, KWD) is off by a factor of a hundred — 500 yen
// sent as "5.00". And it drags the amount through a float64, which is how
// 19.99*100 became 1998 and cost a cent on every captured payment.
//
// That defect was fixed at least six separate times in this tree — in
// currency.CentsFromString, the BitPay and MoonPay decoders, the refund path,
// the capture path, and the webhook settlement writer. It kept coming back
// because nothing stopped it being written again. This is what stops it.
//
// It parses the AST rather than grepping, so the many comments that quote the
// old broken expression to explain the fix do not trip it, and a "%.2f" inside
// a log line does not either.
func TestNoHardcodedMoneyScale(t *testing.T) {
	// Identifiers that name money. A literal 100 against one of these is the
	// defect; a literal 100 against a page size or a percentage is not.
	moneyish := []string{
		"amount", "cents", "price", "total", "subtotal", "fee", "balance",
		"minor", "value", "cost", "credit", "debit", "payout", "refund",
	}

	// Sites that legitimately carry a 100 or a %.2f, each with the reason. A new
	// entry here is a deliberate decision someone has to write down.
	allowed := map[string]string{
		"thirdparty/mercury/api/webhook.go": "a log line, not a wire — the amount it prints is already a float from Mercury's own JSON",
	}

	var offences []string
	for _, root := range []string{"../../payment", "../../thirdparty"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, "../../"))
			if _, ok := allowed[rel]; ok {
				return nil
			}

			fset := token.NewFileSet()
			f, perr := parser.ParseFile(fset, path, nil, 0) // 0 == drop comments
			if perr != nil {
				return nil // a file that does not parse is the compiler's problem, not this test's
			}

			report := func(pos token.Pos, what string) {
				p := fset.Position(pos)
				offences = append(offences, fmt.Sprintf("%s:%d: %s", rel, p.Line, what))
			}

			ast.Inspect(f, func(n ast.Node) bool {
				switch e := n.(type) {
				case *ast.BinaryExpr:
					if e.Op != token.MUL && e.Op != token.QUO {
						return true
					}
					lit, other := scaleLiteral(e.X), e.Y
					if lit == "" {
						lit, other = scaleLiteral(e.Y), e.X
					}
					if lit == "" || !mentionsMoney(other, moneyish) {
						return true
					}
					report(e.Pos(), fmt.Sprintf("%s %s a money value — the scale is Type.Decimals(), never a literal", e.Op, lit))
				case *ast.CallExpr:
					if isLogCall(e) {
						return false
					}
					for _, arg := range e.Args {
						bl, ok := arg.(*ast.BasicLit)
						if !ok || bl.Kind != token.STRING || !strings.Contains(bl.Value, `%.2f`) {
							continue
						}
						for _, rest := range e.Args {
							if rest != arg && mentionsMoney(rest, moneyish) {
								report(bl.Pos(), `"%.2f" renders a money value — two decimals is not every currency's scale`)
								break
							}
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}

	if len(offences) > 0 {
		t.Errorf("money scale hardcoded in %d place(s):\n\t%s\n\n"+
			"Use the currency's own scale: Type.Parse to read text, Type.Amount or money.FromMinor\n"+
			"to build one, Type.ToStringNoSymbol or Amount.MajorString to render it. If a site\n"+
			"genuinely needs the literal, add it to `allowed` in this file with the reason.",
			len(offences), strings.Join(offences, "\n\t"))
	}
}

// scaleLiteral returns the literal's text when it is a decimal-scale constant,
// and "" otherwise. 100 and 0.01 are the two spellings of "assume two decimals".
func scaleLiteral(e ast.Expr) string {
	bl, ok := e.(*ast.BasicLit)
	if !ok {
		return ""
	}
	switch bl.Value {
	case "100", "100.0", "100.00", "0.01":
		return bl.Value
	}
	return ""
}

// mentionsMoney reports whether any identifier in the expression names money.
func mentionsMoney(e ast.Expr, moneyish []string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		lower := strings.ToLower(id.Name)
		for _, m := range moneyish {
			if strings.Contains(lower, m) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// isLogCall reports whether the call is a logger. A log line that prints an
// amount with two decimals is untidy; it is not an instruction to a gateway.
func isLogCall(e *ast.CallExpr) bool {
	sel, ok := e.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch strings.ToLower(pkg.Name) {
	case "log", "logger", "logrus", "slog", "zap":
		return true
	}
	return false
}

// The other half of "the scale comes from the currency" is WHICH parser reads
// it. There is one: currency.Type.Parse, which asks this tree's currency table
// for Decimals(). money.ParseCents pins the scale to USD, and
// money.ParseMinor(s, cur.Money()) is Type.Parse written the long way — so both
// are a second spelling of the sanctioned conversion, and the ParseCents one is
// wrong for any currency that is not two-decimal.
//
// That is not hypothetical: the PayPal IPN amount arrives as "<CURRENCY>
// <decimal>", and parsing the digits with ParseCents read "JPY 500" as 50000
// minor units while the currency sat in the same field.
//
// This walks the whole tree, not just payment/ and thirdparty/, because the
// call sites that had it included api/costs and thirdparty/shipstation.
func TestTypeParseIsTheOnlyDecimalToMinorConversion(t *testing.T) {
	// The two legitimate homes for a direct money.ParseMinor call.
	allowed := map[string]string{
		"models/types/currency/currency.go": "the one implementation — this IS Type.Parse",
		"thirdparty/bitcoin/bitcoin.go":     "a satoshi is 1e-8 BTC, a chain scale this table does not model",
	}

	var offences []string
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, "../../"))
		if _, ok := allowed[rel]; ok {
			return nil
		}

		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "money" {
				return true
			}
			if sel.Sel.Name != "ParseCents" && sel.Sel.Name != "ParseMinor" {
				return true
			}
			p := fset.Position(sel.Pos())
			offences = append(offences, fmt.Sprintf("%s:%d: money.%s", rel, p.Line, sel.Sel.Name))
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}

	if len(offences) > 0 {
		t.Errorf("a second decimal-to-minor parser in %d place(s):\n\t%s\n\n"+
			"Use currency.Type.Parse — it applies THIS table's scale, so a zero-decimal\n"+
			"currency is not read a hundred times too large. If a site genuinely needs a\n"+
			"scale this table does not model, add it to `allowed` in this file with the reason.",
			len(offences), strings.Join(offences, "\n\t"))
	}
}
