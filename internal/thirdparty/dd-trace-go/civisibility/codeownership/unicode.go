// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.
package codeownership

import (
	"unicode"
	"unicode/utf16"
)

// These overrides reconcile Go 1.26 (Unicode 15) and Go 1.27 (Unicode 17)
// with the .NET 10 reference's Unicode 16 classifications and invariant casing.
// They are checked against all BMP chars and every supplementary case mapping.
// See testdata/dotnet-tests.json and scripts/codeownership/README.md.
var dotNetUpperOverrides = map[rune]rune{
	0x0131:  0x0131,
	0x019b:  0xa7dc,
	0x0264:  0xa7cb,
	0x1c8a:  0x1c89,
	0xa7cd:  0xa7cc,
	0xa7cf:  0xa7ce,
	0xa7d3:  0xa7d2,
	0xa7d5:  0xa7d4,
	0xa7db:  0xa7da,
	0x10d70: 0x10d50,
	0x10d71: 0x10d51,
	0x10d72: 0x10d52,
	0x10d73: 0x10d53,
	0x10d74: 0x10d54,
	0x10d75: 0x10d55,
	0x10d76: 0x10d56,
	0x10d77: 0x10d57,
	0x10d78: 0x10d58,
	0x10d79: 0x10d59,
	0x10d7a: 0x10d5a,
	0x10d7b: 0x10d5b,
	0x10d7c: 0x10d5c,
	0x10d7d: 0x10d5d,
	0x10d7e: 0x10d5e,
	0x10d7f: 0x10d5f,
	0x10d80: 0x10d60,
	0x10d81: 0x10d61,
	0x10d82: 0x10d62,
	0x10d83: 0x10d63,
	0x10d84: 0x10d64,
	0x10d85: 0x10d65,
	0x16ebb: 0x16ea0,
	0x16ebc: 0x16ea1,
	0x16ebd: 0x16ea2,
	0x16ebe: 0x16ea3,
	0x16ebf: 0x16ea4,
	0x16ec0: 0x16ea5,
	0x16ec1: 0x16ea6,
	0x16ec2: 0x16ea7,
	0x16ec3: 0x16ea8,
	0x16ec4: 0x16ea9,
	0x16ec5: 0x16eaa,
	0x16ec6: 0x16eab,
	0x16ec7: 0x16eac,
	0x16ec8: 0x16ead,
	0x16ec9: 0x16eae,
	0x16eca: 0x16eaf,
	0x16ecb: 0x16eb0,
	0x16ecc: 0x16eb1,
	0x16ecd: 0x16eb2,
	0x16ece: 0x16eb3,
	0x16ecf: 0x16eb4,
	0x16ed0: 0x16eb5,
	0x16ed1: 0x16eb6,
	0x16ed2: 0x16eb7,
	0x16ed3: 0x16eb8,
}
var dotNetClassOverrides = map[uint16]byte{
	0x088f: 0,
	0x0897: 8,
	0x0c5c: 0,
	0x0cdc: 0,
	0x1acf: 0,
	0x1ad0: 0,
	0x1ad1: 0,
	0x1ad2: 0,
	0x1ad3: 0,
	0x1ad4: 0,
	0x1ad5: 0,
	0x1ad6: 0,
	0x1ad7: 0,
	0x1ad8: 0,
	0x1ad9: 0,
	0x1ada: 0,
	0x1adb: 0,
	0x1adc: 0,
	0x1add: 0,
	0x1ae0: 0,
	0x1ae1: 0,
	0x1ae2: 0,
	0x1ae3: 0,
	0x1ae4: 0,
	0x1ae5: 0,
	0x1ae6: 0,
	0x1ae7: 0,
	0x1ae8: 0,
	0x1ae9: 0,
	0x1aea: 0,
	0x1aeb: 0,
	0x1c89: 1,
	0x1c8a: 1,
	0xa7cb: 1,
	0xa7cc: 1,
	0xa7cd: 1,
	0xa7ce: 0,
	0xa7cf: 0,
	0xa7d2: 0,
	0xa7d4: 0,
	0xa7da: 1,
	0xa7db: 1,
	0xa7dc: 1,
	0xa7f1: 0,
}

func invariantUpper(char rune) rune {
	if char < 128 {
		if char >= 'a' && char <= 'z' {
			return char - ('a' - 'A')
		}
		return char
	}
	if upper, found := dotNetUpperOverrides[char]; found {
		return upper
	}
	return unicode.ToUpper(char)
}
func unitClasses(char uint16) byte {
	if char < 128 {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z':
			return 1
		case char >= '0' && char <= '9':
			return 2
		case char == ' ', char >= '\t' && char <= '\r':
			return 4
		case char == '_':
			return 8
		default:
			return 0
		}
	}
	if flags, found := dotNetClassOverrides[char]; found {
		return flags
	}
	r := rune(char)
	var flags byte
	if unicode.IsLetter(r) {
		flags |= 1
	}
	if unicode.IsDigit(r) {
		flags |= 2
	}
	if unicode.IsSpace(r) {
		flags |= 4
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Pc, r) {
		flags |= 8
	}
	return flags
}

// utf16Units uses caller-owned scratch space for ordinary short tokens. Longer
// input grows through append; callers must not retain a view of reused storage.
func utf16Units(value string, storage []uint16) []uint16 {
	units := storage[:0]
	for _, char := range value {
		if char > 0xffff {
			high, low := utf16.EncodeRune(char)
			units = append(units, uint16(high), uint16(low))
		} else {
			units = append(units, uint16(char))
		}
	}
	return units
}
