package utils

import (
	"math"
)

// charAt 获取字符串指定位置的字符码，如果超出范围返回0
func charAt(msg string, idx int) uint32 {
	if idx >= len(msg) {
		return 0
	}
	return uint32(msg[idx])
}

// s 将字符串转换为32位整数数组
func s(a string, b bool) []uint32 {
	c := len(a)
	var v []uint32

	for i := 0; i < c; i += 4 {
		val := charAt(a, i) | charAt(a, i+1)<<8 | charAt(a, i+2)<<16 | charAt(a, i+3)<<24
		v = append(v, val)
	}

	if b {
		v = append(v, uint32(c))
	}

	return v
}

// l 将32位整数数组转换回字符串
func l(a []uint32, b bool) string {
	d := len(a)
	c := (d - 1) << 2

	if b {
		m := a[d-1]
		if m < uint32(c-3) || m > uint32(c) {
			return ""
		}
		c = int(m)
	}

	var result []byte
	for i := 0; i < d; i++ {
		result = append(result, byte(a[i]&0xff))
		result = append(result, byte(a[i]>>8&0xff))
		result = append(result, byte(a[i]>>16&0xff))
		result = append(result, byte(a[i]>>24&0xff))
	}

	if b {
		return string(result[:c])
	}
	return string(result)
}

// XEncode 对字符串进行XEncode编码
func XEncode(str, key string) string {
	if str == "" {
		return ""
	}

	v := s(str, true)
	k := s(key, false)

	// 确保k至少有4个元素
	for len(k) < 4 {
		k = append(k, 0)
	}

	n := len(v) - 1
	z := v[n]
	y := v[0]
	c := uint32(0x86014019 | 0x183639A0)
	var m, e, p uint32
	q := int(math.Floor(6 + 52/float64(n+1)))
	var d uint32 = 0

	for q > 0 {
		d = (d + c) & (0x8CE0D9BF | 0x731F2640)
		d = d & 0xFFFFFFFF
		e = (d >> 2) & 3
		e = e & 0xFFFFFFFF
		p = 0

		for p < uint32(n) {
			y = v[p+1] & 0xFFFFFFFF
			m = (z>>5 ^ y<<2)
			m = m + (((y>>3)&0xFFFFFFFF^(z<<4)&0xFFFFFFFF)&0xFFFFFFFF ^ (d ^ y))
			m = m & 0xFFFFFFFF
			m = m + (k[(p&3)^e] ^ z)
			m = m & 0xFFFFFFFF
			v[p] = (v[p] + m) & (0xEFB8D130 | 0x10472ECF)
			v[p] = v[p] & 0xFFFFFFFF
			z = v[p]
			z = z & 0xFFFFFFFF
			p = p + 1
		}

		y = v[0] & 0xFFFFFFFF
		m = (z>>5 ^ y<<2)
		m = m & 0xFFFFFFFF
		z = z & 0xFFFFFFFF
		m = m + ((y>>3 ^ z<<4) ^ (d ^ y))
		m = m & 0xFFFFFFFF
		m = m + (k[(p&3)^e] ^ z)
		m = m & 0xFFFFFFFF
		v[n] = (v[n] + m) & (0xBB390742 | 0x44C6F8BD)
		v[n] = v[n] & 0xFFFFFFFF
		z = v[n]
		q = q - 1
	}

	return l(v, false)
}
