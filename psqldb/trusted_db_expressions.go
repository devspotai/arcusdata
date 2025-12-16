package psqldb

import "fmt"

type Expr struct{ s string } // trusted only, not user input

func (e Expr) String() string { return e.s }

func ExprNow() Expr { return Expr{s: "NOW()"} }

// Optional: forbid unsafe tokens even in trusted exprs, as a guardrail.
// Keep it strict and small.
func safeExpr(e Expr) error {
	if e.s == "" {
		return fmt.Errorf("empty expression")
	}
	// could add: reject ';', '--', '/*' etc if you want a hard guardrail
	return nil
}
