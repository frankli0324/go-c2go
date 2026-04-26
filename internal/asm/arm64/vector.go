package arm64

import (
	"fmt"
	"strconv"
	"strings"
)

func floatRegister(name string) (string, error) {
	n, _, err := parseVecReg(name, "qdv")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("F%d", n), nil
}

func vectorRegisterLane(arg, lane string) (string, error) {
	n, argLane, err := parseVecReg(arg, "v")
	if err != nil {
		return "", err
	}
	if argLane != "" {
		lane = normalizeLane(argLane)
	}
	reg := fmt.Sprintf("V%d", n)
	if lane != "" {
		reg += "." + lane
	}
	return reg, nil
}

func parseVecReg(arg, prefixes string) (int, string, error) {
	arg = strings.ToLower(strings.TrimSpace(arg))
	if len(arg) < 2 || !strings.ContainsRune(prefixes, rune(arg[0])) {
		return 0, "", fmt.Errorf("unsupported arm64 vector register %q", arg)
	}
	end := 1
	for end < len(arg) && arg[end] >= '0' && arg[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(arg[1:end])
	if err != nil || n < 0 || n > 31 {
		return 0, "", fmt.Errorf("unsupported arm64 vector register %q", arg)
	}
	return n, strings.ToUpper(strings.TrimPrefix(arg[end:], ".")), nil
}

func isVectorOp(op string, args []string) bool {
	op = strings.ToLower(op)
	if base, _, ok := strings.Cut(op, "."); ok {
		return isVectorOpName(base)
	}
	if len(args) == 0 || !strings.Contains(args[0], ".") {
		return false
	}
	return isVectorOpName(op)
}

func isVectorOpName(op string) bool {
	switch op {
	case "add", "eor", "orr", "ushll", "ushll2", "ext", "ushl":
		return true
	default:
		return false
	}
}

func vectorOp(op string, args []string) (string, error) {
	lower := strings.ToLower(op)
	if !strings.Contains(lower, ".") && len(args) > 0 {
		_, lane, err := parseVecReg(args[0], "v")
		if err != nil {
			return "", err
		}
		lower += "." + strings.ToLower(lane)
	}
	lane := opLane(lower)
	switch {
	case strings.HasPrefix(lower, "ushll") && len(args) == 3:
		return vectorUshll(lower, lane, args)
	case strings.HasPrefix(lower, "ext.") && len(args) == 4:
		regs, err := vectorRegList([]string{lane}, args[:3]...)
		if err != nil {
			return "", err
		}
		return "VEXT " + mustOperand(args[3]) + ", " + regs[2] + ", " + regs[1] + ", " + regs[0], nil
	case len(args) == 3:
		if lower == "ushl.2d" {
			return vectorUshl2D(args)
		}
		regs, err := vectorRegList([]string{lane}, args...)
		if err != nil {
			return "", err
		}
		return "V" + strings.ToUpper(strings.Split(lower, ".")[0]) + " " + regs[2] + ", " + regs[1] + ", " + regs[0], nil
	default:
		return "", fmt.Errorf("unsupported arm64 vector op %q", op)
	}
}

func vectorMove(op string, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("unsupported arm64 vector move")
	}
	lane := ""
	if suffix, ok := strings.CutPrefix(strings.ToLower(op), "mov."); ok {
		lane = normalizeLane(suffix)
	}
	if isVectorArg(args[0]) {
		src, err := vectorMoveOperand(args[1], args[0], lane)
		if err != nil {
			return "", err
		}
		dst, err := vectorMoveOperand(args[0], args[1], lane)
		if err != nil {
			return "", err
		}
		return "VMOV " + src + ", " + dst, nil
	}
	out, err := operand(args[0])
	if err != nil {
		return "", err
	}
	return "VMOV " + vectorElement(args[1]) + ", " + out, nil
}

func isVectorArg(arg string) bool {
	arg = strings.ToLower(strings.TrimSpace(arg))
	return strings.HasPrefix(arg, "v") || strings.HasPrefix(arg, "q") || strings.HasPrefix(arg, "d")
}

func vectorMoveOperand(arg, peer, lane string) (string, error) {
	arg = strings.TrimSpace(arg)
	if strings.HasPrefix(strings.ToLower(arg), "x") || strings.HasPrefix(strings.ToLower(arg), "w") {
		return operand(arg)
	}
	if strings.HasPrefix(strings.ToLower(arg), "q") || strings.HasPrefix(strings.ToLower(arg), "d") {
		return vectorElement(arg), nil
	}
	if strings.Contains(arg, "[") {
		return vectorElement(arg), nil
	}
	_, parsedLane, err := parseVecReg(arg, "v")
	if err != nil {
		return "", err
	}
	if parsedLane != "" {
		lane = parsedLane
	}
	if lane == "" {
		_, peerLane, peerErr := parseVecReg(peer, "v")
		if peerErr == nil {
			lane = peerLane
		}
	}
	return vectorRegisterLane(arg, lane)
}

func vectorUshll(op, lane string, args []string) (string, error) {
	srcLane := narrowLane(lane)
	if strings.HasPrefix(op, "ushll2") {
		srcLane = doubleLane(srcLane)
	}
	regs, err := vectorRegList([]string{lane, srcLane}, args[0], args[1])
	if err != nil {
		return "", err
	}
	return "V" + strings.ToUpper(strings.Split(op, ".")[0]) + " " + mustOperand(args[2]) + ", " + regs[1] + ", " + regs[0], nil
}

func vectorRegList(lanes []string, args ...string) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		lane := lanes[0]
		if len(lanes) > 1 {
			lane = lanes[i]
		}
		reg, err := vectorRegisterLane(arg, lane)
		if err != nil {
			return nil, err
		}
		out[i] = reg
	}
	return out, nil
}

func vectorUshl2D(args []string) (string, error) {
	var reg [3]int
	for i, arg := range args {
		n, _, err := parseVecReg(arg, "v")
		if err != nil {
			return "", err
		}
		reg[i] = n
	}
	word := uint32(0x6ee04400) | uint32(reg[2])<<16 | uint32(reg[1])<<5 | uint32(reg[0])
	return fmt.Sprintf("WORD $0x%08x", word), nil
}

func opLane(op string) string {
	_, suffix, _ := strings.Cut(op, ".")
	return normalizeLane(suffix)
}

func normalizeLane(lane string) string {
	lane = strings.ToUpper(lane)
	i := strings.IndexFunc(lane, func(r rune) bool { return r < '0' || r > '9' })
	if i < 0 {
		return lane
	}
	return lane[i:] + lane[:i]
}

func narrowLane(lane string) string {
	if lane == "" {
		return ""
	}
	if i := strings.IndexByte("HSD", lane[0]); i >= 0 {
		return string("BHS"[i]) + lane[1:]
	}
	return lane
}

func doubleLane(lane string) string {
	if len(lane) < 2 {
		return lane
	}
	count, err := strconv.Atoi(lane[1:])
	if err != nil {
		return lane
	}
	return string(lane[0]) + strconv.Itoa(count*2)
}

func vectorElement(arg string) string {
	arg = strings.ToUpper(strings.TrimSpace(arg))
	if strings.HasPrefix(arg, "D") {
		return "V" + strings.TrimPrefix(arg, "D") + ".D[0]"
	}
	if strings.HasPrefix(arg, "Q") {
		return "V" + strings.TrimPrefix(arg, "Q") + ".B16"
	}
	if strings.HasPrefix(arg, "V") && strings.Contains(arg, "[") && !strings.Contains(arg, ".") {
		return strings.Replace(arg, "[", ".D[", 1)
	}
	return arg
}
