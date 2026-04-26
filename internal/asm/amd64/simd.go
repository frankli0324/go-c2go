package amd64

import "strings"

func isSIMDSuffixMnemonic(lower string) bool {
	for _, suffix := range simdSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

var simdSuffixes = []string{
	"ss", "sd", "ps", "pd", "dq", "dqa", "dqu",
}

var avxThreeOpWhitelist = map[string]struct{}{
	"vaddps": {}, "vaddpd": {},
	"vsubps": {}, "vsubpd": {},
	"vmulps": {}, "vmulpd": {},
	"vdivps": {}, "vdivpd": {},
	"vmaxps": {}, "vmaxpd": {},
	"vmaxss": {}, "vmaxsd": {},
	"vminps": {}, "vminpd": {},
	"vminss": {}, "vminsd": {},
	"vandps": {}, "vandpd": {},
	"vandnps": {}, "vandnpd": {},
	"vorps": {}, "vorpd": {},
	"vxorps": {}, "vxorpd": {},
	"vaddsubps": {}, "vaddsubpd": {},
	"vhaddps": {}, "vhaddpd": {},
	"vhsubps": {}, "vhsubpd": {},
	"vaddss": {}, "vaddsd": {},
	"vsubss": {}, "vsubsd": {},
	"vmulss": {}, "vmulsd": {},
	"vdivss": {}, "vdivsd": {},
	"vcvtss2sd": {}, "vcvtsd2ss": {},
}

func isWhitelistedAVXThreeOp(op string) bool {
	_, ok := avxThreeOpWhitelist[strings.ToLower(op)]
	return ok
}
