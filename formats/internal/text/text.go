// Package text holds the lenient MUGEN-style text helpers Ikemen GO uses for
// every text format (AIR, CNS, CMD, DEF, ...). Sharing them keeps number
// parsing identical to the engine across all parsers.
//
// Upstream: ikemen/source/src/common.go @ v1.0.0 (81c6da71d689625e20db79586815b695da00dd6d)
//
//	IsNumeric (common.go:149), Atoi (common.go:174), Atof (common.go:205),
//	LoadText BOM handling (common.go:396), SplitAndTrim (common.go:692),
//	SectionName (common.go:755)
//
// Changes from upstream: Atoi reports int32 overflow through its second
// return value instead of writing to the engine console/log.
//
// MIT License
// Copyright (c) 2016 Suehiro
// Copyright (c) 2018 K4thos, NeatUnsou, ShinLucho
// Copyright (c) 2019 Daniel Porto, Gacel, Nikolay Griva
// See ikemen/source/LICENCE.txt for the full notice.
package text

import (
	"math"
	"strconv"
	"strings"
)

const (
	IMax = int32(^uint32(0) >> 1)
	IErr = ^IMax
)

// IsNumeric reports whether s parses as a float.
func IsNumeric(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

// Atoi parses a leading integer and ignores everything after it ("12abc" = 12,
// "" = 0). On int32 overflow it returns IMax (or IErr for negatives) and
// overflow = true.
func Atoi(str string) (n32 int32, overflow bool) {
	var n int64
	str = strings.TrimSpace(str)
	if len(str) > 0 {
		var a string
		if str[0] == '-' || str[0] == '+' {
			a = str[1:]
		} else {
			a = str
		}
		for i := range a {
			if a[i] < '0' || '9' < a[i] {
				break
			}
			n = n*10 + int64(a[i]-'0')
			if n > 2147483647 {
				if str[0] == '-' {
					return IErr, true
				}
				return IMax, true
			}
		}
		if str[0] == '-' {
			n *= -1
		}
	}
	return int32(n), false
}

// Atof parses a leading float the same lenient way.
func Atof(str string) float64 {
	f := 0.0
	str = strings.TrimSpace(str)
	if len(str) > 0 {
		var a string
		if str[0] == '-' || str[0] == '+' {
			a = str[1:]
		} else {
			a = str
		}
		i, p := 0, 0
		for ; i < len(a); i++ {
			if a[i] == '.' {
				if p != 0 {
					break
				}
				p = i + 1
				continue
			}
			if a[i] < '0' || '9' < a[i] {
				break
			}
			f = f*10 + float64(a[i]-'0')
		}
		e := 0.0
		if i+1 < len(a) && (a[i] == 'e' || a[i] == 'E') {
			j := i + 1
			if a[j] == '-' || a[j] == '+' {
				j++
			}
			for ; j < len(a) && '0' <= a[j] && a[j] <= '9'; j++ {
				e = e*10 + float64(a[j]-'0')
			}
			if e != 0 {
				if str[i+1] == '-' {
					e *= -1
				}
				if p == 0 {
					p = i
				}
			}
		}
		if p > 0 {
			f *= math.Pow10(p - i + int(e))
		}
		if str[0] == '-' {
			f *= -1
		}
	}
	return f
}

// StripBOM removes a leading UTF-8 byte order mark.
func StripBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
		return b[3:]
	}
	return b
}

// SplitAndTrim splits str on sep and trims white space from each part.
func SplitAndTrim(str, sep string) (ss []string) {
	ss = strings.Split(str, sep)
	for i, s := range ss {
		ss[i] = strings.TrimSpace(s)
	}
	return
}

// SectionName splits "[Begin Action 200]" into ("begin ", "Action 200").
// The name is lower-cased and keeps its trailing space; the rest is returned
// as written. Lines that are not a section return "", "".
func SectionName(sec string) (string, string) {
	if len(sec) == 0 || sec[0] != '[' {
		return "", ""
	}
	sec = strings.TrimSpace(strings.SplitN(sec, ";", 2)[0])
	if sec[len(sec)-1] != ']' {
		return "", ""
	}
	sec = sec[1:strings.Index(sec, "]")]
	var name string
	i := strings.Index(sec, " ")
	if i >= 0 {
		name = sec[:i+1]
		sec = sec[i+1:]
	} else {
		name = sec
		sec = ""
	}
	return strings.ToLower(name), sec
}
