package asm

import (
	"strconv"
	"strings"
)

var pseudoComment = map[string]struct{}{
	".globl": {}, ".global": {}, ".type": {}, ".size": {}, ".section": {}, ".align": {}, ".text": {}, ".data": {}, ".bss": {},
}

func translateLines(src string, v translator) (string, int) {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	unsupported := 0
	for _, raw := range lines {
		converted, bad := translateLine(raw, v)
		out = append(out, converted...)
		if bad {
			unsupported++
		}
	}
	return strings.Join(out, "\n"), unsupported
}

func translateLine(raw string, v translator) ([]string, bool) {
	indent := leadingWhitespace(raw)
	body := splitComment(raw, v.CommentPrefix())
	trimmed := strings.TrimSpace(body)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") {
		return []string{""}, false
	}

	label, rest := splitLabel(trimmed)
	var out []string
	if label != "" {
		out = append(out, indent+label+":")
		trimmed = strings.TrimSpace(rest)
		if trimmed == "" {
			return out, false
		}
	}

	if strings.HasPrefix(trimmed, ".") {
		line, drop, unsupported := handlePseudo(indent, trimmed)
		if drop {
			return out, unsupported
		}
		out = append(out, line)
		return out, unsupported
	}

	line, unsupported := v.TranslateInstruction(indent, trimmed)
	out = append(out, line)
	return out, unsupported
}

func splitComment(raw, prefix string) string {
	body := raw
	if prefix != "" {
		if idx := strings.Index(body, prefix); idx >= 0 {
			body = body[:idx]
		}
	}
	if prefix != "//" {
		if idx := strings.Index(body, "//"); idx >= 0 {
			body = body[:idx]
		}
	}
	return body
}

func splitLabel(line string) (label, rest string) {
	if idx := strings.Index(line, ":"); idx >= 0 {
		candidate := strings.TrimSpace(line[:idx])
		if candidate != "" && !strings.Contains(candidate, " ") && !strings.Contains(candidate, "\t") {
			return candidate, line[idx+1:]
		}
	}
	return "", line
}

func handlePseudo(indent, line string) (string, bool, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", true, false
	}
	op := strings.ToLower(fields[0])
	if strings.HasPrefix(op, ".cfi_") || op == ".loc" || op == ".file" || op == ".loh" || op == ".ident" || op == ".addrsig" || op == ".build_version" || op == ".subsections_via_symbols" {
		return "", true, false
	}
	if op == ".p2align" {
		args := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		align, _, _ := strings.Cut(args, ",")
		align = strings.TrimSpace(align)
		if align == "" {
			align = "0"
		}
		shift, err := strconv.Atoi(align)
		if err != nil || shift < 3 || shift > 11 {
			return indent + "// " + line, false, false
		}
		return indent + "PCALIGN $" + strconv.Itoa(1<<shift), false, false
	}
	switch op {
	case ".byte", ".short", ".word", ".long", ".quad", ".xword":
		return indent + op + " " + strings.TrimSpace(strings.TrimPrefix(line, fields[0])), false, false
	}
	if _, ok := pseudoComment[op]; ok {
		return indent + "// " + line, false, false
	}
	return indent + "// UNSUPPORTED: " + line, false, true
}

func leadingWhitespace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}
