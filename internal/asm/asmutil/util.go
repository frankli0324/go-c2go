package asmutil

import (
	"strconv"
	"strings"
)

type Translator interface {
	CommentPrefix() string
	TranslateInstruction(indent, line string) (string, bool)
}

func JoinInstruction(mnemonic string, operands []string) string {
	if len(operands) == 0 {
		return mnemonic
	}
	return mnemonic + " " + strings.Join(operands, ", ")
}

func SplitOperands(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var parts []string
	depthParen := 0
	depthBracket := 0
	start := 0
	for i, r := range s {
		switch r {
		case '(':
			depthParen++
		case ')':
			if depthParen > 0 {
				depthParen--
			}
		case '[':
			depthBracket++
		case ']':
			if depthBracket > 0 {
				depthBracket--
			}
		case ',':
			if depthParen == 0 && depthBracket == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	filtered := parts[:0]
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func AddSB(sym string) string {
	sym = strings.TrimSpace(sym)
	if sym == "" {
		return sym
	}
	if strings.Contains(sym, "(SB)") || IsNumeric(sym) {
		return sym
	}
	return sym + "(SB)"
}

func IsNumeric(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if strings.HasSuffix(strings.ToLower(v), "h") {
		_, err := strconv.ParseInt(strings.TrimSuffix(strings.ToLower(v), "h"), 16, 64)
		return err == nil
	}
	_, err := strconv.ParseInt(v, 0, 64)
	return err == nil
}

func IsLocalLabel(label string) bool {
	label = strings.TrimSpace(label)
	return strings.HasPrefix(label, ".L") || strings.HasPrefix(label, "L")
}
