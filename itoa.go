package itoa

import (
	"math/bits"
)

//https://github.com/appnexus/acf/blob/master/src/an_itoa.c

const (
	PO2           uint64 = 100
	PO4           uint64 = 10000
	PO8           uint64 = 100000000
	PO10          uint64 = 10000000000
	PO16          uint64 = 10000000000000000
	ZERO_CHARS    uint64 = 0x3030303030303030
	ZERO_CHARS_32 uint32 = 0x30303030
	ZERO_MASK     uint32 = 0xFFFFFFF8

	MAX_UINT64_DIGITS = 20
)

//
//https://github.com/cznic/mathutil
//

func AddUint128_64(a, b uint64) (hi uint64, lo uint64) {
	lo = a + b
	if lo < a {
		hi = 1
	}
	return
}

func MulUint128_64(a, b uint64) (hi, lo uint64) {
	/*
		2^(2 W) ahi bhi + 2^W alo bhi + 2^W ahi blo + alo blo
		FEDCBA98 76543210 FEDCBA98 76543210
		                  ---- alo*blo ----
		         ---- alo*bhi ----
		         ---- ahi*blo ----
		---- ahi*bhi ----
	*/
	const w = 32
	const m = 1<<w - 1
	ahi, bhi, alo, blo := a>>w, b>>w, a&m, b&m
	lo = alo * blo
	mid1 := alo * bhi
	mid2 := ahi * blo
	c1, lo := AddUint128_64(lo, mid1<<w)
	c2, lo := AddUint128_64(lo, mid2<<w)
	_, hi = AddUint128_64(ahi*bhi, mid1>>w+mid2>>w+c1+c2)
	return
}

//

func encodeHundreds(hi, lo uint32) uint32 {
	/*
	 * Pack everything in a single 32 bit value.
	 *
	 * merged = [ hi 0 lo 0 ]
	 */
	merged := hi | (lo << 16)
	/*
	 * Fixed-point multiplication by 103/1024 ~= 1/10.
	 */
	tens := (merged * 103) >> 10

	/*
	 * Mask away garbage bits between our digits.
	 *
	 * tens = [ hi/10 0 lo/10 0 ]
	 *
	 * On a platform with more restricted literals (ARM, for
	 * instance), it may make sense to and-not with the middle
	 * bits.
	 */
	tens &= (0xF << 16) | 0xF

	/*
	 * x mod 10 = x - 10 * (x div 10).
	 *
	 * (merged - 10 * tens) = [ hi%10 0 lo%10 0 ]
	 *
	 * Then insert these values between tens.  Arithmetic instead
	 * of bitwise operation helps the compiler merge this with
	 * later increments by a constant (e.g., ZERO_CHARS).
	 */
	return tens + ((merged - 10*tens) << 8)
}

/**
 * SWAR encode 10000 hi + lo to byte (unpacked) BCD.
 */
func encodeTenThousands(hi, lo uint64) uint64 {
	merged := hi | (lo << 32)
	/* Truncate division by 100: 10486 / 2**20 ~= 1/100. */
	top := ((merged * 10486) >> 20) & ((0x7F << 32) | 0x7F)
	/* Trailing 2 digits in the 1e4 chunks. */
	bot := merged - 100*top

	/*
	 * We now have 4 radix-100 digits in little-endian order, each
	 * in its own 16 bit area.
	 */
	hundreds := (bot << 16) + top

	/* Divide and mod by 10 all 4 radix-100 digits in parallel. */
	tens := (hundreds * 103) >> 10
	tens &= (0xF << 48) | (0xF << 32) | (0xF << 16) | 0xF
	tens += (hundreds - 10*tens) << 8

	return tens
}

/**
 * Range-specialised version of itoa.
 *
 * We always convert to fixed-width BCD then shift away any leading
 * zero.  The slop will manifest as writing zero bytes after our
 * encoded string, which is acceptable: we never write more than the
 * maximal length (10 or 20 characters).
 */

/**
 * itoa for x < 100.
 */
func itoaHundred(out []byte, x uint32) []byte {
	/*
	 * -1 if x < 10, 0 otherwise.  Tried to get an sbb, but ?:
	 * gets us branches.
	 */
	//small := uint32((uint64(x) + uint64(0xfffffffffffffff6)) >> 8)
	small := uint32((int32(x - 10)) >> 8)
	//base := (unsigned int)'0' | ((unsigned int)'0' << 8);
	base := uint32(0x30 | (0x30 << 8))
	/*
	 * Probably not necessary, but why not abuse smaller constants?
	 * Also, see block comment above idiv_POx functions.
	 */
	hi := (x * 103) >> 10
	lo := x - 10*hi

	base += hi + (lo << 8)
	/* Shift away the leading zero (shift by 8) if x < 10. */
	base = base >> uint(small&8)
	//memcpy(out, &base, 2)
	out[0] = byte(base)
	out[1] = byte(base >> 8)

	/* 2 + small = 1 if x < 10, 2 otherwise. */
	return out[2+small:]
}

/**
 * itoa for x < 10k.
 */
func itoaTenThousand(out []byte, x uint32) []byte {
	xDivPO2 := (x * 10486) >> 20
	xModPO2 := x - uint32(PO2)*xDivPO2
	buf := encodeHundreds(xDivPO2, xModPO2)
	/*
	 * Count leading (in memory, trailing in register: we're
	 * little endian) zero bytes: count leading zero bits and
	 * round down to 8.
	 */
	zeros := uint32(bits.TrailingZeros32(buf)) & ZERO_MASK
	buf += ZERO_CHARS_32 /* BCD -> ASCII. */
	buf = buf >> zeros   /* Shift away leading zero characters */

	//	memcpy(out, &buf, 4);
	out[0] = byte(buf)
	out[1] = byte(buf >> 8)
	out[2] = byte(buf >> 16)
	out[3] = byte(buf >> 24)

	/* zeros is in bits; convert to bytes to find actual length. */
	return out[4-zeros/8:]
}

/**
 * 32 bit helpers for truncation by constant.
 *
 * We only need them because GCC is stupid with likely/unlikely
 * annotations: unlikely code is compiled with an extreme emphasis on
 * size, up to compiling integer division by constants to actual div
 * instructions.  In turn, we want likely annotations because we only
 * get a nice ladder of forward conditional jumps when there is no
 * code between if blocks.  We convince GCC that our "special" cases
 * for shorter integers aren't slowpathed guards by marking each
 * conditional as likely.
 *
 * The constants are easily proven correct (or compared with those
 * generated by a reference compiler, e.g., GCC or clang).  For
 * example,
 *
 *   1/10000 ~= k = 3518437209 / 2**45 = 1/10000 + 73/21990232555520000.
 *
 * Let eps = 73/21990232555520000; for any 0 <= x < 2**32,
 * floor(k * x) <= floor(x / 10000 + x * eps)
 *              <= floor(x / 10000 + 2**32 * eps)
 *              <= floor(x / 10000 + 2e-5).
 *
 * Given that x is unsigned, flooring the left and right -hand sides
 * will yield the same value as long as the error term
 * (x * eps <= 2e-5) is less than 1/10000, and 2e-5 < 10000.  We finally
 * conclude that 3518437209 / 2**45, our fixed point approximation of
 * 1/10000, is always correct for truncated division of 32 bit
 * unsigned ints.
 */

/**
 * Divide a 32 bit int by 1e4.
 */
func idivPO4(x uint32) uint32 {
	wide := uint64(x)
	mul := uint64(3518437209)

	return uint32((wide * mul) >> 45)
}

/**
 * Divide a 32 bit int by 1e8.
 */
func idivPO8(x uint32) uint64 {
	wide := uint64(x)
	mul := uint64(1441151881)

	return (wide * mul) >> 57
}

func copy8(out []byte, buf uint64) {
	out[0] = byte(buf)
	out[1] = byte(buf >> 8)
	out[2] = byte(buf >> 16)
	out[3] = byte(buf >> 24)
	out[4] = byte(buf >> 32)
	out[5] = byte(buf >> 40)
	out[6] = byte(buf >> 48)
	out[7] = byte(buf >> 56)
}
func anItoa(out []byte, x uint32) []byte {

	/*
	 * Smaller numbers can be encoded more quickly.  Special
	 * casing them makes a significant difference compared to
	 * always going through 8-digit encoding.
	 */
	if x < uint32(PO2) {
		return itoaHundred(out, x)
	}

	if x < uint32(PO4) {
		return itoaTenThousand(out, x)
	}

	/*
	 * Manual souped up common subexpression elimination.
	 *
	 * The sequel always needs x / PO4 and x % PO4.  Compute them
	 * here, before branching.  We may also need x / PO8 if
	 * x >= PO8.  Benchmarking shows that performing this division
	 * by constant unconditionally doesn't hurt.  If x >= PO8, we'll
	 * always want x_div_PO4 = (x % PO8) / PO4.  We compute that
	 * in a roundabout manner to reduce the makespan, i.e., the
	 * length of the dependency chain for (x % PO8) % PO4 = x % PO4.
	 */
	xDivPO4 := idivPO4(x)
	xModPO4 := x - xDivPO4*uint32(PO4)
	xDivPO8 := uint32(idivPO8(x))
	/*
	 * We actually want x_div_PO4 = (x % PO8) / PO4.
	 * Subtract what would have been removed by (x % PO8) from
	 * x_div_PO4.
	 */
	xDivPO4 -= xDivPO8 * uint32(PO4)
	/*
	 * Finally, we can unconditionally encode_ten_thousands the
	 * values we obtain after division by PO8 and fixup by
	 * x_div_PO8 * PO4.
	 */
	buf := encodeTenThousands(uint64(xDivPO4), uint64(xModPO4))

	if x < uint32(PO8) {

		zeros := uint32(bits.TrailingZeros64(buf)) & ZERO_MASK

		buf += ZERO_CHARS
		buf = buf >> zeros

		//memcpy(out, &buf, 8);
		out[0] = byte(buf)
		out[1] = byte(buf >> 8)
		out[2] = byte(buf >> 16)
		out[3] = byte(buf >> 24)
		out[4] = byte(buf >> 32)
		out[5] = byte(buf >> 40)
		out[6] = byte(buf >> 48)
		out[7] = byte(buf >> 56)

		return out[8-zeros/8:]
	}

	/* 32 bit integers are always below 1e10. */
	buf += ZERO_CHARS
	out = itoaHundred(out, xDivPO8)

	//memcpy(out, &buf, 8);
	copy8(out, buf)
	return out[8:]
}

/**
 * 64 bit helpers for truncation by constant.
 */

/**
 * Divide a 64 bit int by 1e4.
 */
func ldivPO4(x uint64) uint64 {
	hi, _ := MulUint128_64(x, uint64(3777893186295716171))
	return hi >> 11
}

/**
 * Divide a 64 bit int by 1e8.
 */
func ldivPO8(x uint64) uint64 {
	hi, _ := MulUint128_64(x, uint64(12379400392853802749))

	return hi >> 26
}

/**
 * Divide a 64 bit int by 1e16.
 */
func ldivPO16(x uint64) uint64 {
	hi, _ := MulUint128_64(x, uint64(4153837486827862103))
	return hi >> 51
}

// FormatUint returns a byte slice containing a uint64 formatted to a string.
// Note that this uses some intermediate memory; if you want to target a binary
// format, use Anltoa.
func FormatUint(x uint64) string {
	var buf [MAX_UINT64_DIGITS]byte
	remainder := Anltoa(buf[:], x)
	return string(buf[:len(buf)-len(remainder)])
}

// FormatInt returns a byte slice containing a int64 formatted to a string.
// Note that this uses some intermediate memory; if you want to target a binary
// format, use Anltoa.
func FormatInt(x int64) string {
	if x >= 0 {
		return FormatUint(uint64(x))
	}

	var buf [MAX_UINT64_DIGITS + 1]byte
	buf[0] = '-'
	remainder := Anltoa(buf[1:], uint64(-x))
	return string(buf[:len(buf)-len(remainder)])
}

// Anltoa takes a byte buffer and a number, and encodes the number as a string
// to the buffer. It will panic if the buffer is too small; the maximum number
// of digits a 64-bit uint can be is 20. It returns the portion of the slice
// after the encoded number.
func Anltoa(out []byte, x uint64) []byte {
	if x < PO2 {
		return itoaHundred(out, uint32(x))
	}

	if x < PO4 {
		return itoaTenThousand(out, uint32(x))
	}

	xDivPO4 := ldivPO4(x)
	xModPO4 := x - xDivPO4*PO4

	/*
	 * Benchmarking shows the long division by PO8 hurts
	 * performance for PO4 <= x < PO8.  Keep encode_ten_thousands
	 * conditional for an_ltoa.
	 */
	if x < PO8 {
		buf := encodeTenThousands(xDivPO4, xModPO4)
		zeros := uint32(bits.TrailingZeros64(buf)) & ZERO_MASK
		buf += ZERO_CHARS
		buf = buf >> zeros

		copy8(out, buf)
		return out[8-zeros/8:]
	}

	/* See block comment in an_itoa. */
	xDivPO8 := ldivPO8(x)
	xDivPO4 = xDivPO4 - xDivPO8*PO4
	buf := encodeTenThousands(xDivPO4, xModPO4) + ZERO_CHARS

	/*
	 * Add a case for PO8 <= x < PO10 because itoa_hundred is much
	 * quicker than a second call to encode_ten_thousands; the
	 * same isn't true of itoa_ten_thousand.
	 */
	if x < PO10 {
		out = itoaHundred(out, uint32(xDivPO8))
		copy8(out, buf)
		return out[8:]
	}

	/*
	 * Again, long division by PO16 hurts, so do the rest
	 * conditionally.
	 */
	if x < PO16 {
		/* x_div_PO8 < PO8 < 2**32, so idiv_PO4 is safe. */
		hiHi := uint64(idivPO4(uint32(xDivPO8)))
		hiLo := xDivPO8 - hiHi*PO4
		bufHi := encodeTenThousands(hiHi, hiLo)
		zeros := uint32(bits.TrailingZeros64(bufHi)) & ZERO_MASK

		bufHi += ZERO_CHARS
		bufHi = bufHi >> zeros

		copy8(out, bufHi)
		i := 8 - zeros/8
		copy8(out[i:], buf)
		return out[i+8:]
	}

	hi := ldivPO16(x)
	mid := xDivPO8 - hi*PO8
	midHi := uint64(idivPO4(uint32(mid)))
	midLo := mid - midHi*PO4
	bufMid := encodeTenThousands(midHi, midLo) + ZERO_CHARS

	out = itoaTenThousand(out, uint32(hi))
	copy8(out, bufMid)
	copy8(out[8:], buf)
	return out[16:]
}
