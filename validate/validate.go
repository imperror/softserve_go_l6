// Package validate provides simple syntactic validators for common
// user-input formats.
//
// Homework — Task 2 (Lesson 6: File I/O, JSON and Testing):
// Implement ValidateEmail and/or ValidatePhone below (your mentor may
// ask for just one) and extend the test tables in validate_test.go to
// at least 8 cases each, including edge cases.
package validate

import (
	"strings"
	"unicode"
)

const (
	maxEmailLocalLength  = 64
	maxDomainLabelLength = 63
	maxDomainLength      = 253
	minTLDLength         = 2
	minPhoneDigits       = 7
	maxPhoneDigits       = 15
)

// ValidateEmail reports whether s is a syntactically valid email address.
//
// TODO: implement this function. At minimum it should:
//   - reject the empty string,
//   - require exactly one "@" with a non-empty local part and domain part,
//   - reject values containing whitespace.
//
// Document any additional decisions you make (e.g. how you handle a
// trailing dot, consecutive dots, or unicode characters) in a comment
// here, and add matching test cases in validate_test.go.
func ValidateEmail(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsFunc(s, unicode.IsSpace) {
		return false
	}

	at := strings.IndexByte(s, '@')
	if at < 0 || strings.ContainsRune(s[at+1:], '@') {
		return false
	}

	local, domain := s[:at], s[at+1:]
	if !validEmailLocal(local) || !validEmailDomain(domain) {
		return false
	}
	return true
}

func validEmailLocal(local string) bool {
	if local == "" || len(local) > maxEmailLocalLength {
		return false
	}
	if local[0] == '.' || local[len(local)-1] == '.' || strings.Contains(local, "..") {
		return false
	}
	for _, r := range local {
		if !isASCIIAlnum(r) && r != '.' && r != '_' && r != '+' && r != '-' {
			return false
		}
	}
	return true
}

func validEmailDomain(domain string) bool {
	if domain == "" || len(domain) > maxDomainLength {
		return false
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}

	last := len(labels) - 1
	for i, label := range labels {
		if label == "" || len(label) > maxDomainLabelLength {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		if i == last && (len(label) < minTLDLength || !allASCIILetters(label)) {
			return false
		}
		for _, r := range label {
			if !isASCIIAlnum(r) && r != '-' {
				return false
			}
		}
	}
	return true
}

func isASCIIAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func allASCIILetters(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			if r < 'A' || r > 'Z' {
				return false
			}
		}
	}
	return true
}

// ValidatePhone reports whether s is a syntactically valid phone number.
//
// TODO: implement this function. Decide which format(s) you accept
// (e.g. "+380501234567", "050-123-4567") and document your decision
// here. At minimum it should reject the empty string and any value
// containing letters.
func ValidatePhone(s string) bool {
	if s == "" {
		return false
	}
	if strings.ContainsFunc(s, unicode.IsLetter) {
		return false
	}

	digits := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+':
			if i != 0 {
				return false
			}
		case r == '-' || r == '.' || r == ' ' || r == '(' || r == ')':
			continue
		default:
			return false
		}
	}

	return digits >= minPhoneDigits && digits <= maxPhoneDigits
}
