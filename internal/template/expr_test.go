package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalExpr_SimpleVar(t *testing.T) {
	result, err := EvalExpr("i", map[string]int{"i": 5})
	require.NoError(t, err)
	assert.Equal(t, 5, result)
}

func TestEvalExpr_Arithmetic(t *testing.T) {
	result, err := EvalExpr("i*7", map[string]int{"i": 3})
	require.NoError(t, err)
	assert.Equal(t, 21, result)
}

func TestEvalExpr_Parentheses(t *testing.T) {
	result, err := EvalExpr("(i-1)*7", map[string]int{"i": 3})
	require.NoError(t, err)
	assert.Equal(t, 14, result)
}

func TestEvalExpr_Complex(t *testing.T) {
	result, err := EvalExpr("i*7-1", map[string]int{"i": 3})
	require.NoError(t, err)
	assert.Equal(t, 20, result)
}

func TestEvalExpr_IntegerLiteral(t *testing.T) {
	result, err := EvalExpr("42", map[string]int{})
	require.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestExpandTemplate_Simple(t *testing.T) {
	result, err := ExpandTemplate("Week {i}", map[string]int{"i": 5})
	require.NoError(t, err)
	assert.Equal(t, "Week 5", result)
}

func TestExpandTemplate_MultipleVars(t *testing.T) {
	result, err := ExpandTemplate("w{i}_s{j}", map[string]int{"i": 3, "j": 2})
	require.NoError(t, err)
	assert.Equal(t, "w3_s2", result)
}

func TestExpandTemplate_Expression(t *testing.T) {
	result, err := ExpandTemplate("{(i-1)*7}", map[string]int{"i": 3})
	require.NoError(t, err)
	assert.Equal(t, "14", result)
}

func TestExpandTemplate_NoVars(t *testing.T) {
	result, err := ExpandTemplate("Static Title", map[string]int{})
	require.NoError(t, err)
	assert.Equal(t, "Static Title", result)
}

// ============ NEGATIVE TEST CASES ============

func TestEvalExpr_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		vars    map[string]int
		wantErr string
	}{
		// Empty expression
		{"empty expression", "", map[string]int{"i": 5}, "empty expression"},

		// Undefined variables
		{"undefined variable single", "x", map[string]int{"i": 5}, "undefined variable: x"},
		{"undefined variable in expression", "i+x", map[string]int{"i": 5}, "undefined variable: x"},
		{"undefined variable after multiply", "x*3", map[string]int{}, "undefined variable: x"},

		// Unexpected characters
		{"ampersand operator", "i&j", map[string]int{"i": 5, "j": 3}, "unexpected character"},
		{"division not supported", "i/2", map[string]int{"i": 10}, "unexpected character"},
		{"percent operator", "i%2", map[string]int{"i": 5}, "unexpected character"},
		{"caret operator", "i^2", map[string]int{"i": 2}, "unexpected character"},

		// Double operators
		{"double plus", "i++1", map[string]int{"i": 5}, "unexpected character"},
		{"double minus", "i--1", map[string]int{"i": 5}, "unexpected character"},
		{"star before var", "*i", map[string]int{"i": 5}, "unexpected character"},
		{"plus at start", "+i", map[string]int{"i": 5}, "unexpected character"},

		// Trailing operators
		{"trailing plus", "i+", map[string]int{"i": 5}, "unexpected end of expression"},
		{"trailing multiply", "i*", map[string]int{"i": 5}, "unexpected end of expression"},
		{"trailing minus", "i-", map[string]int{"i": 5}, "unexpected end of expression"},

		// Unmatched parentheses
		{"missing close paren", "(i+1", map[string]int{"i": 5}, "expected ')'"},
		{"extra close paren", "i+1)", map[string]int{"i": 5}, "unexpected character"},
		{"nested missing close", "((i+1)", map[string]int{"i": 5}, "expected ')'"},
		{"empty parens", "()", map[string]int{}, "unexpected character"},
		{"open paren at end", "i*(", map[string]int{"i": 5}, "unexpected end of expression"},

		// Complex malformed expressions
		{"operator after close paren", "(i+1)*2)", map[string]int{"i": 5}, "unexpected character"},
		{"space with operator", "i &", map[string]int{"i": 5}, "unexpected character"},
		{"multiple spaces", "i  +  *j", map[string]int{"i": 5, "j": 3}, "unexpected character"},

		// Variable-like but not valid
		{"number starting with letter", "1a", map[string]int{}, "unexpected character"},
		{"dot operator", "i.j", map[string]int{"i": 5, "j": 3}, "unexpected character"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EvalExpr(tc.expr, tc.vars)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Equal(t, 0, result, "should return zero on error")
		})
	}
}

func TestExpandTemplate_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		vars    map[string]int
		wantErr string
	}{
		// Unmatched braces
		{"unmatched open brace at end", "Week {i", map[string]int{"i": 5}, "unmatched '{'"},
		{"unmatched open brace mid", "Week {i something", map[string]int{"i": 5}, "unmatched '{'"},
		{"nested unmatched brace", "Week {{i}", map[string]int{"i": 5}, "unmatched '{'"},

		// Empty braces
		{"empty braces", "Week {}", map[string]int{}, "empty expression"},
		{"only spaces in braces", "Week { }", map[string]int{}, "empty expression"},

		// Undefined variables in template
		{"undefined var in template", "Week {x}", map[string]int{"i": 5}, "undefined variable: x"},
		{"undefined var in expression", "{x+1}", map[string]int{"i": 5}, "undefined variable: x"},

		// Malformed expressions in template
		{"division in template", "Week {i/2}", map[string]int{"i": 5}, "unexpected character"},
		{"trailing operator in template", "Week {i*}", map[string]int{"i": 5}, "unexpected end of expression"},
		{"double operator in template", "Week {i++1}", map[string]int{"i": 5}, "unexpected character"},
		{"unmatched paren in template", "Week {(i+1}", map[string]int{"i": 5}, "expected ')'"},

		// Multiple expressions
		{"multiple braces with error", "{i} and {x}", map[string]int{"i": 5}, "undefined variable: x"},
		{"first expr ok second error", "{i} {x}", map[string]int{"i": 5}, "undefined variable: x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExpandTemplate(tc.tmpl, tc.vars)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
			assert.Equal(t, "", result, "should return empty string on error")
		})
	}
}

// ============ EDGE CASES - BOUNDARY TESTING ============

func TestEvalExpr_BoundaryValues(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		vars     map[string]int
		expected int
	}{
		// Zero and negative values
		{"zero value", "i", map[string]int{"i": 0}, 0},
		{"multiply by zero", "i*0", map[string]int{"i": 42}, 0},
		{"zero minus positive", "0-i", map[string]int{"i": 5}, -5},
		{"negative result", "i-10", map[string]int{"i": 3}, -7},

		// Large numbers
		{"large literal", "999999", map[string]int{}, 999999},
		{"large variable", "i", map[string]int{"i": 999999}, 999999},
		{"large multiplication", "1000*1000", map[string]int{}, 1000000},

		// Complex nesting
		{"triple nested parens", "(((i)))", map[string]int{"i": 5}, 5},
		{"complex expression", "((i-1)*2)+(3*4)", map[string]int{"i": 10}, 30},
		{"many operations", "1+2*3-4+5*2", map[string]int{}, 13},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EvalExpr(tc.expr, tc.vars)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExpandTemplate_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		vars     map[string]int
		expected string
	}{
		// No substitutions
		{"no braces", "plain text", map[string]int{}, "plain text"},
		{"multiple words no braces", "this is a test", map[string]int{}, "this is a test"},

		// Multiple valid expressions
		{"two expressions", "{i}-{j}", map[string]int{"i": 5, "j": 3}, "5-3"},
		{"expression and literal", "Item {i}: {i*2} minutes", map[string]int{"i": 30}, "Item 30: 60 minutes"},

		// Edge case: braces not as expression
		{"no space after open", "{i}test", map[string]int{"i": 1}, "1test"},
		{"consecutive braces", "{i}{j}", map[string]int{"i": 1, "j": 2}, "12"},
		{"brace at start", "{i} test", map[string]int{"i": 5}, "5 test"},
		{"brace at end", "test {i}", map[string]int{"i": 5}, "test 5"},

		// Zero and negative in template
		{"zero in template", "Value: {i}", map[string]int{"i": 0}, "Value: 0"},
		{"negative in template", "Minus: {i-10}", map[string]int{"i": 3}, "Minus: -7"},

		// Complex expression
		{"complex calc", "Total: {(i+j)*2}", map[string]int{"i": 5, "j": 10}, "Total: 30"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ExpandTemplate(tc.tmpl, tc.vars)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
