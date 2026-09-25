package psqldb

import (
	"fmt"
	"regexp"
	"strings"
)

type Op string

const (
	OpEqual              Op = "="
	OpNotEqual           Op = "!="
	OpGreaterThan        Op = ">"
	OpLessThan           Op = "<"
	OpIn                 Op = "IN"
	OpLike               Op = "LIKE"
	OpILike              Op = "ILIKE"
	OpGreaterThanOrEqual Op = ">="
	OpLessThanOrEqual    Op = "<="
)

var identifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validOps is the closed set of operators that may be interpolated into SQL.
// Op is a string type, so callers can construct arbitrary values with Op(s);
// anything not in this set must be rejected rather than rendered.
var validOps = map[Op]struct{}{
	OpEqual: {}, OpNotEqual: {}, OpGreaterThan: {}, OpLessThan: {},
	OpIn: {}, OpLike: {}, OpILike: {}, OpGreaterThanOrEqual: {}, OpLessThanOrEqual: {},
}

// ValidateOp reports whether op is one of the defined Op constants.
func ValidateOp(op Op) error {
	if _, ok := validOps[op]; !ok {
		return fmt.Errorf("unsafe SQL operator %q", string(op))
	}
	return nil
}

func ValidateIdentifier(name string) (string, error) {
	if !identifierRegex.MatchString(name) {
		return "", fmt.Errorf("invalid SQL identifier %q", name)
	}
	return name, nil
}

// CTEContext is the minimal contract AuthPermissionsCTE needs.
// All builders (query/update/insert/delete) will implement this.
type CTEContext interface {
	Param(v any) string
	AddCTE(cteDef string)
	QuoteIdentifier(name string) (string, error)
	QuoteDottedIdentifier(name string) (string, error)
	Err() error
	SetErr(err error)
	// ApplyAuthGuard attaches the EXISTS guard that references the named CTE.
	// It exists so AuthPermissionsCTE.Apply can enforce itself instead of
	// relying on the caller to remember a second call.
	ApplyAuthGuard(cteName string)
}

// BuilderCore centralizes args, ctes, identifier quoting, and error handling.
// Embed it into all builders.
type BuilderCore struct {
	args []any
	ctes []string
	err  error
}

func (c *BuilderCore) Err() error { return c.err }
func (c *BuilderCore) SetErr(err error) {
	if c.err == nil {
		c.err = err
	}
}

func (c *BuilderCore) Args() []any { return c.args }

// Param appends a parameter and returns $n placeholder.
func (c *BuilderCore) Param(v any) string {
	if c.err != nil {
		return "$0"
	}
	c.args = append(c.args, v)
	return fmt.Sprintf("$%d", len(c.args))
}

func (c *BuilderCore) AddCTE(cteDef string) {
	if c.err != nil {
		return
	}
	cteDef = strings.TrimSpace(cteDef)
	if cteDef == "" {
		c.SetErr(fmt.Errorf("cte definition cannot be empty"))
		return
	}
	c.ctes = append(c.ctes, cteDef)
}

func (c *BuilderCore) QuoteIdentifier(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("identifier cannot be empty")
	}
	if !identifierRegex.MatchString(name) {
		return "", fmt.Errorf("unsafe identifier %q", name)
	}
	return `"` + name + `"`, nil
}

// QuoteDottedIdent quotes schema.table or alias.column: a.b -> "a"."b".
func (c *BuilderCore) QuoteDottedIdentifier(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("identifier cannot be empty")
	}
	parts := strings.Split(name, ".")
	for _, p := range parts {
		if !identifierRegex.MatchString(p) {
			return "", fmt.Errorf("unsafe identifier %q", name)
		}
	}
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, "."), nil
}
