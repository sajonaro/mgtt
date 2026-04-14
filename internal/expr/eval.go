package expr

import (
	"errors"
	"fmt"
	"strconv"
)

// Eval implementations — And/Or use short-circuit semantics with the twist
// that an UnresolvedError is propagated only when the other side is true.

func (n AndNode) Eval(ctx Ctx) (bool, error) {
	lv, lerr := n.L.Eval(ctx)
	rv, rerr := n.R.Eval(ctx)
	if lerr != nil && rerr != nil {
		return false, lerr
	}
	if lerr != nil {
		if !rv {
			return false, nil
		}
		return false, lerr
	}
	if rerr != nil {
		if !lv {
			return false, nil
		}
		return false, rerr
	}
	return lv && rv, nil
}

func (n OrNode) Eval(ctx Ctx) (bool, error) {
	lv, lerr := n.L.Eval(ctx)
	rv, rerr := n.R.Eval(ctx)
	if lerr != nil && rerr != nil {
		return false, lerr
	}
	if lerr != nil {
		if rv {
			return true, nil
		}
		return false, lerr
	}
	if rerr != nil {
		if lv {
			return true, nil
		}
		return false, rerr
	}
	return lv || rv, nil
}

func (n CmpNode) Eval(ctx Ctx) (bool, error) {
	component := n.Component
	if component == "" {
		component = ctx.CurrentComponent
	}

	if n.Fact == "state" {
		stateVal, ok := ctx.States[component]
		if !ok {
			return false, &UnresolvedError{Component: component, Fact: "state", Reason: "missing"}
		}
		return compareStrings(n.Op, stateVal, n.Value)
	}

	factVal, ok := ctx.Facts.LookupValue(component, n.Fact)
	if !ok {
		return false, &UnresolvedError{Component: component, Fact: n.Fact, Reason: "missing"}
	}
	return compareFactValue(n.Op, factVal, n.Value, component, ctx)
}

// compareFactValue compares a runtime fact value against the parsed literal.
// A non-numeric string on the RHS is re-interpreted as a fact reference in
// the same component (e.g. "ready_replicas < desired_replicas").
func compareFactValue(op CmpOp, factVal any, nodeVal any, component string, ctx Ctx) (bool, error) {
	if bv, ok := factVal.(bool); ok {
		nb, err := asBool(nodeVal)
		if err != nil {
			return false, &UnresolvedError{Component: component, Reason: "type mismatch"}
		}
		return compareBools(op, bv, nb)
	}

	// String fact — numeric strings participate in numeric comparisons.
	if sv, ok := factVal.(string); ok {
		if f, err := strconv.ParseFloat(sv, 64); err == nil {
			if nodeFloat, nerr := toFloat(nodeVal); nerr == nil {
				return compareFloats(op, f, nodeFloat), nil
			}
		}
		ns, err := asString(nodeVal)
		if err != nil {
			return false, &UnresolvedError{Component: component, Reason: "type mismatch"}
		}
		return compareStrings(op, sv, ns)
	}

	factFloat, ok := numeric(factVal)
	if !ok {
		return false, fmt.Errorf("unsupported fact type %T", factVal)
	}

	if nodeFloat, err := toFloat(nodeVal); err == nil {
		return compareFloats(op, factFloat, nodeFloat), nil
	}
	// RHS is a non-numeric string — treat it as an identifier referring to
	// another fact in the same component.
	if s, ok := nodeVal.(string); ok {
		rhsVal, ok := ctx.Facts.LookupValue(component, s)
		if !ok {
			return false, &UnresolvedError{Component: component, Fact: s, Reason: "missing"}
		}
		rhsFloat, ok := numeric(rhsVal)
		if !ok {
			return false, &UnresolvedError{Component: component, Fact: s, Reason: "type mismatch"}
		}
		return compareFloats(op, factFloat, rhsFloat), nil
	}
	return false, &UnresolvedError{Component: component, Reason: "type mismatch"}
}

// numeric converts int/int64/float32/float64 fact values to float64. Other
// integer widths never appear — facts come from YAML decode or JSON parse.
func numeric(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// toFloat converts a parsed Value (int, float64, or numeric string) to float64.
func toFloat(v any) (float64, error) {
	switch x := v.(type) {
	case int:
		return float64(x), nil
	case float64:
		return x, nil
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, nil
		}
		if i, err := strconv.Atoi(x); err == nil {
			return float64(i), nil
		}
	}
	return 0, errors.New("cannot convert value to number")
}

func compareFloats(op CmpOp, a, b float64) bool {
	switch op {
	case OpEq:
		return a == b
	case OpNeq:
		return a != b
	case OpLt:
		return a < b
	case OpGt:
		return a > b
	case OpLte:
		return a <= b
	case OpGte:
		return a >= b
	}
	return false
}

func compareBools(op CmpOp, a, b bool) (bool, error) {
	switch op {
	case OpEq:
		return a == b, nil
	case OpNeq:
		return a != b, nil
	}
	return false, fmt.Errorf("operator %v not valid for bool", op)
}

func compareStrings(op CmpOp, factStr string, nodeVal any) (bool, error) {
	s, err := asString(nodeVal)
	if err != nil {
		return false, err
	}
	switch op {
	case OpEq:
		return factStr == s, nil
	case OpNeq:
		return factStr != s, nil
	}
	return false, fmt.Errorf("operator %v not valid for string", op)
}

func asBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		if x == "true" {
			return true, nil
		}
		if x == "false" {
			return false, nil
		}
	}
	return false, fmt.Errorf("value is not bool")
}

func asString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case int:
		return strconv.Itoa(x), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	}
	return "", fmt.Errorf("value has no string representation")
}
