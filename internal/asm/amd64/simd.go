package amd64

import (
	"strings"

	"golang.org/x/arch/x86/x86asm"
)

func init() {
	addExactAs := func(mn string, names ...string) {
		for _, name := range names {
			opSpecs[name] = opSpec{typ: opExact, mn: mn}
		}
	}
	addExact := func(op x86asm.Op) {
		addExactAs(op.String(), strings.ToLower(op.String()))
	}
	addVEXExact := func(op x86asm.Op) {
		addExactAs("V"+op.String(), "v"+strings.ToLower(op.String()))
	}
	addExactAs("MASKMOVOU", "maskmovdqu")
	addExactAs("MOVO", "movdqa")
	addExactAs("MOVOU", "movdqu")
	addExactAs("MOVNTO", "movntdq")
	addExactAs("MOVSD", "movsd")
	addExactAs("VEXTRACTI128", "vextracti128")
	addExactAs("VINSERTI128", "vinserti128")
	addExactAs("VPBROADCASTQ", "vpbroadcastq")
	addExactAs("VPERMQ", "vpermq")
	addExact(x86asm.MOVAPD)
	addExact(x86asm.MOVAPS)
	addExact(x86asm.MOVSS)
	addExact(x86asm.MOVUPD)
	addExact(x86asm.MOVUPS)
	addExact(x86asm.PSHUFB)
	addExact(x86asm.PXOR)
	addExact(x86asm.CVTSS2SD)
	addExact(x86asm.CVTSD2SS)
	addExact(x86asm.CVTSS2SI)
	addExact(x86asm.CVTSD2SI)
	addExact(x86asm.CVTTSS2SI)
	addExact(x86asm.CVTTSD2SI)
	addVEXExact(x86asm.PEXTRQ)
	addVEXExact(x86asm.PSHUFD)
	addExact(x86asm.VMOVDQA)
	addExact(x86asm.VMOVDQU)
	addExact(x86asm.VZEROUPPER)

	addSuffix := func(ops ...x86asm.Op) {
		for _, op := range ops {
			name := strings.ToLower(op.String())
			if _, ok := opSpecs[name]; !ok {
				opSpecs[name] = opSpec{typ: opExact, mn: op.String()}
			}
			if _, ok := opSpecs["v"+name]; !ok {
				opSpecs["v"+name] = opSpec{typ: opExact, mn: "V" + op.String()}
			}
		}
	}
	addAVX3 := func(ops ...x86asm.Op) {
		addSuffix(ops...)
		for _, op := range ops {
			name := strings.ToLower(op.String())
			opSpecs["v"+name] = opSpec{typ: opAVX3, mn: "V" + op.String()}
		}
	}
	addSuffix(
		x86asm.BLENDPD, x86asm.BLENDPS, x86asm.BLENDVPD, x86asm.BLENDVPS,
		x86asm.CMPPD, x86asm.CMPPS, x86asm.CMPSD_XMM, x86asm.CMPSS,
		x86asm.COMISD, x86asm.COMISS,
		x86asm.CVTDQ2PD, x86asm.CVTDQ2PS, x86asm.CVTPD2DQ, x86asm.CVTPD2PS,
		x86asm.CVTPI2PD, x86asm.CVTPI2PS, x86asm.CVTPS2DQ, x86asm.CVTPS2PD,
		x86asm.CVTSI2SD, x86asm.CVTSI2SS, x86asm.CVTTPD2DQ, x86asm.CVTTPS2DQ,
		x86asm.DPPD, x86asm.DPPS, x86asm.EXTRACTPS, x86asm.INSERTPS,
		x86asm.LDDQU, x86asm.MOVHLPS, x86asm.MOVHPD, x86asm.MOVHPS,
		x86asm.MOVLHPS, x86asm.MOVLPD, x86asm.MOVLPS, x86asm.MOVMSKPD,
		x86asm.MOVMSKPS, x86asm.MOVNTDQA, x86asm.MOVNTPD, x86asm.MOVNTPS,
		x86asm.MOVNTSD, x86asm.MOVNTSS, x86asm.MOVQ2DQ, x86asm.PABSD,
		x86asm.PCLMULQDQ, x86asm.PMOVSXDQ, x86asm.PMOVZXDQ, x86asm.PMULDQ,
		x86asm.PSLLDQ, x86asm.PSRLDQ, x86asm.PUNPCKHDQ, x86asm.PUNPCKHQDQ,
		x86asm.PUNPCKLDQ, x86asm.PUNPCKLQDQ, x86asm.RCPPS, x86asm.RCPSS,
		x86asm.ROUNDPD, x86asm.ROUNDPS, x86asm.ROUNDSD, x86asm.ROUNDSS,
		x86asm.RSQRTPS, x86asm.RSQRTSS, x86asm.SHUFPD, x86asm.SHUFPS,
		x86asm.SQRTPD, x86asm.SQRTPS, x86asm.SQRTSD, x86asm.SQRTSS,
		x86asm.UCOMISD, x86asm.UCOMISS, x86asm.UNPCKHPD, x86asm.UNPCKHPS,
		x86asm.UNPCKLPD, x86asm.UNPCKLPS,
	)
	addExact(x86asm.VMOVNTDQ)
	addExact(x86asm.VMOVNTDQA)
	addAVX3(
		x86asm.ADDPS, x86asm.ADDPD, x86asm.SUBPS, x86asm.SUBPD,
		x86asm.MULPS, x86asm.MULPD, x86asm.DIVPS, x86asm.DIVPD,
		x86asm.MAXPS, x86asm.MAXPD, x86asm.MAXSS, x86asm.MAXSD,
		x86asm.MINPS, x86asm.MINPD, x86asm.MINSS, x86asm.MINSD,
		x86asm.ANDPS, x86asm.ANDPD, x86asm.ANDNPS, x86asm.ANDNPD,
		x86asm.ORPS, x86asm.ORPD, x86asm.XORPS, x86asm.XORPD,
		x86asm.ADDSUBPS, x86asm.ADDSUBPD, x86asm.HADDPS, x86asm.HADDPD,
		x86asm.HSUBPS, x86asm.HSUBPD, x86asm.ADDSS, x86asm.ADDSD,
		x86asm.SUBSS, x86asm.SUBSD, x86asm.MULSS, x86asm.MULSD,
		x86asm.DIVSS, x86asm.DIVSD, x86asm.CVTSS2SD, x86asm.CVTSD2SS,
		x86asm.PADDB, x86asm.PADDW, x86asm.PADDD, x86asm.PADDQ,
		x86asm.PSUBB, x86asm.PSUBW, x86asm.PSUBD, x86asm.PSUBQ,
		x86asm.PMULLW, x86asm.PMULLD, x86asm.PMULUDQ,
		x86asm.PAND, x86asm.PANDN, x86asm.POR, x86asm.PXOR,
		x86asm.PSLLW, x86asm.PSLLD, x86asm.PSLLQ,
		x86asm.PSRLW, x86asm.PSRLD, x86asm.PSRLQ,
		x86asm.PSRAW, x86asm.PSRAD,
		x86asm.PCMPEQB, x86asm.PCMPEQW, x86asm.PCMPEQD, x86asm.PCMPEQQ,
		x86asm.PCMPGTB, x86asm.PCMPGTW, x86asm.PCMPGTD, x86asm.PCMPGTQ,
		x86asm.PMAXSB, x86asm.PMAXSW, x86asm.PMAXSD,
		x86asm.PMAXUB, x86asm.PMAXUW, x86asm.PMAXUD,
		x86asm.PMINSB, x86asm.PMINSW, x86asm.PMINSD,
		x86asm.PMINUB, x86asm.PMINUW, x86asm.PMINUD,
		x86asm.PAVGB, x86asm.PAVGW, x86asm.PSHUFB,
	)
}
