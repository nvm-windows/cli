package constant

import (
	"common/settings"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
)

// RelaxDeadlines is --relax-deadlines.
// A bare flag triples configured budgets. A positive integer is milliseconds.
// Fields stay unexported so kong treats the value as one flag.
type RelaxDeadlines struct {
	active       bool
	milliseconds int
}

func (r RelaxDeadlines) Active() bool { return r.active }

func (r RelaxDeadlines) Milliseconds() int { return r.milliseconds }

func (r RelaxDeadlines) Setting() settings.RelaxDeadlines {
	return settings.RelaxDeadlines{Active: r.active, Milliseconds: r.milliseconds}
}

func (r *RelaxDeadlines) Decode(ctx *kong.DecodeContext) error {
	r.active = true
	raw, ok := tokenString(ctx.Scan.Peek().Value)
	if !ok || raw == "" || strings.HasPrefix(raw, "-") {
		return nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	if n <= 0 {
		return fmt.Errorf("--relax-deadlines must be a positive number of milliseconds")
	}
	ctx.Scan.Pop()
	r.milliseconds = n
	return nil
}

func tokenString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}
