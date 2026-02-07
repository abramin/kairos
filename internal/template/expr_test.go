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
