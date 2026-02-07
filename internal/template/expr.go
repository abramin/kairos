package template

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// EvalExpr evaluates a simple arithmetic expression with variables.
// Supports: integer literals, variable names, +, -, *, parentheses.
// Variables are resolved from the provided map.
// Example: "(i-1)*7" with vars {"i": 3} => 14
func EvalExpr(expr string, vars map[string]int) (int, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	p := &parser{input: expr, vars: vars}
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %c", p.pos, p.input[p.pos])
	}
	return result, nil
}

type parser struct {
	input string
	pos   int
	vars  map[string]int
}

func (p *parser) parseExpr() (int, error) {
	return p.parseAddSub()
}

func (p *parser) parseAddSub() (int, error) {
	left, err := p.parseMul()
	if err != nil {
		return 0, err
	}

	for p.pos < len(p.input) {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		op := p.input[p.pos]
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right, err := p.parseMul()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseMul() (int, error) {
	left, err := p.parseAtom()
	if err != nil {
		return 0, err
	}

	for p.pos < len(p.input) {
		p.skipSpaces()
		if p.pos >= len(p.input) {
			break
		}
		if p.input[p.pos] != '*' {
			break
		}
		p.pos++
		right, err := p.parseAtom()
		if err != nil {
			return 0, err
		}
		left *= right
	}
	return left, nil
}

func (p *parser) parseAtom() (int, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	ch := p.input[p.pos]

	// Parenthesized expression
	if ch == '(' {
		p.pos++ // skip '('
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("expected ')' at position %d", p.pos)
		}
		p.pos++ // skip ')'
		return val, nil
	}

	// Number
	if unicode.IsDigit(rune(ch)) {
		start := p.pos
		for p.pos < len(p.input) && unicode.IsDigit(rune(p.input[p.pos])) {
			p.pos++
		}
		val, err := strconv.Atoi(p.input[start:p.pos])
		if err != nil {
			return 0, err
		}
		return val, nil
	}

	// Variable name (letters and underscores)
	if unicode.IsLetter(rune(ch)) || ch == '_' {
		start := p.pos
		for p.pos < len(p.input) && (unicode.IsLetter(rune(p.input[p.pos])) || unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '_') {
			p.pos++
		}
		name := p.input[start:p.pos]
		val, ok := p.vars[name]
		if !ok {
			return 0, fmt.Errorf("undefined variable: %s", name)
		}
		return val, nil
	}

	return 0, fmt.Errorf("unexpected character '%c' at position %d", ch, p.pos)
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

// ExpandTemplate expands a template string by replacing {expr} blocks with evaluated results.
// Example: "Week {i}" with vars {"i": 3} => "Week 3"
// Example: "{(i-1)*7}" with vars {"i": 3} => "14"
func ExpandTemplate(tmpl string, vars map[string]int) (string, error) {
	var result strings.Builder
	i := 0
	for i < len(tmpl) {
		if tmpl[i] == '{' {
			// Find matching closing brace
			j := i + 1
			depth := 1
			for j < len(tmpl) && depth > 0 {
				if tmpl[j] == '{' {
					depth++
				} else if tmpl[j] == '}' {
					depth--
				}
				j++
			}
			if depth != 0 {
				return "", fmt.Errorf("unmatched '{' at position %d", i)
			}
			expr := tmpl[i+1 : j-1]
			val, err := EvalExpr(expr, vars)
			if err != nil {
				return "", fmt.Errorf("evaluating expression '%s': %w", expr, err)
			}
			result.WriteString(strconv.Itoa(val))
			i = j
		} else {
			result.WriteByte(tmpl[i])
			i++
		}
	}
	return result.String(), nil
}
