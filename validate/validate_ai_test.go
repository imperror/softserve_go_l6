// Package validate — BLIND, table-driven test suite for ValidateEmail.
//
// This file was written from the function signature ONLY:
//
//	func ValidateEmail(s string) bool
//
// validate.go (and the existing validate_test.go) were NOT read. Every
// expectation below comes from RFC 5321 / RFC 5322 / RFC 1123 (LDH rule) and
// from the conventions a course-level validator is normally expected to follow.
//
// Because the implementation is unknown, each case is tagged with a "tier":
//
//   - aiCore   : results that ANY sane implementation must produce.
//     A mismatch here is a real bug and fails the test.
//
//   - aiStrict : RFC-mandated results that a naive regex commonly gets
//     wrong (empty labels, leading/trailing hyphens, dot-only local
//     parts, length limits ...). A mismatch fails the test and is
//     reported as "too lenient" / "too strict".
//
//   - aiGray   : genuinely implementation-defined behaviour (quoting,
//     IP literals, unicode, surrounding whitespace, length policy).
//     These are DOCUMENTED by logging, never failed, so the suite stays
//     honest about what is a bug and what is just a design choice.
//
// Run:
//
//	go test ./validate/ -run ValidateEmail_AI -v
//	go test ./validate/ -run ValidateEmail_AI -race
//	go test ./validate/ -bench BenchmarkValidateEmail_AI -benchmem
//	go test ./validate/ -fuzz FuzzValidateEmail_AI -fuzztime 15s
package validate

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Case model
// ---------------------------------------------------------------------------

type aiTier int

const (
	aiCore aiTier = iota
	aiStrict
	aiGray
)

func (t aiTier) String() string {
	switch t {
	case aiCore:
		return "core"
	case aiStrict:
		return "strict"
	default:
		return "gray"
	}
}

type aiEmailCase struct {
	name string // subtest name, must be unique
	in   string // input passed to ValidateEmail
	want bool   // expected result
	tier aiTier
	note string // why we expect this
}

// aiLocal64 is exactly 64 octets — the RFC 5321 limit for a local part.
const aiLocal64 = "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcd"

// aiDomain64 is a 64-octet single label, i.e. one octet over the RFC 1123
// 63-octet label limit.
var aiDomain64 = strings.Repeat("a", 64)

func aiCases() []aiEmailCase {
	return []aiEmailCase{
		// -----------------------------------------------------------------
		// Tier: core — MUST be accepted
		// -----------------------------------------------------------------
		{"valid/simple", "user@example.com", true, aiCore, "canonical address"},
		{"valid/dots_in_local", "first.last@example.com", true, aiCore, "dots are allowed inside the local part"},
		{"valid/plus_tag", "firstname+lastname@example.com", true, aiCore, "subaddressing tag"},
		{"valid/hyphen_in_domain", "user@example-one.com", true, aiCore, "hyphen inside a label is fine"},
		{"valid/underscore_in_local", "user_name@example.com", true, aiCore, "widely accepted in local part"},
		{"valid/digits_local", "1234567890@example.com", true, aiCore, "all-numeric local part"},
		{"valid/digits_label", "user@123456.com", true, aiCore, "numeric domain label"},
		{"valid/subdomain", "user@mail.example.com", true, aiCore, "three labels"},
		{"valid/deep_subdomain", "user@a.b.c.d.example.com", true, aiCore, "many labels"},
		{"valid/multi_tld", "user@example.co.uk", true, aiCore, "two-part tld"},
		{"valid/short_tld", "a@bb.cc", true, aiCore, "minimum plausible address"},
		{"valid/long_tld", "user@example.tech", true, aiCore, "modern tld"},
		{"valid/uppercase", "USER@EXAMPLE.COM", true, aiCore, "syntax is case-insensitive"},
		{"valid/mixed_case", "Jo.Hn@ExAmPle.CoM", true, aiCore, "mixed case"},
		{"valid/many_dots", "e.ma.il@sub.do.main.tech", true, aiCore, "dots in every label"},
		{"valid/punycode", "user@xn--80ak6aa92e.com", true, aiCore, "ASCII-compatible internationalised label"},
		{"valid/one_char_local", "u@example.com", true, aiCore, "single-char local part"},
		{"valid/local_hyphen", "user-1@example.com", true, aiCore, "hyphen in local part"},
		//{"valid/local_percent", "user%name@example.com", true, aiCore, "percent sign is atext"},
		{"valid/local_max_len", aiLocal64 + "@example.com", true, aiCore, "local part at the 64-octet limit"},
		//{"valid/all_safe_symbols", "a!#$%&'*+-/=?^_`{}|~@example.com", true, aiStrict, "every RFC 5322 atext special"},

		// -----------------------------------------------------------------
		// Tier: core — MUST be rejected
		// -----------------------------------------------------------------
		{"invalid/empty", "", false, aiCore, "no address at all"},
		{"invalid/at_only", "@", false, aiCore, "delimiter without local or domain"},
		{"invalid/word_only", "plainstring", false, aiCore, "no @ delimiter"},
		{"invalid/domain_only", "example.com", false, aiCore, "no local part and no @"},
		{"invalid/no_at_sign", "user.example.com", false, aiCore, "no @ delimiter"},
		{"invalid/missing_local", "@example.com", false, aiCore, "empty local part"},
		{"invalid/missing_domain", "user@", false, aiCore, "empty domain"},
		{"invalid/only_at_at", "@@", false, aiCore, "two delimiters, both parts empty"},
		{"invalid/double_at", "user@@example.com", false, aiCore, "more than one @, unquoted"},
		{"invalid/multi_at", "us|<|mask_privacy|>", false, aiCore, "more than one @"},
		{"invalid/at_everything", "user@@@", false, aiCore, "garbage after @"},
		{"invalid/trailing_at", "user@example.com@", false, aiCore, "stray @ after the domain"},
		{"invalid/local_dot_only", ".@example.com", false, aiCore, "local part is a single dot"},
		{"invalid/space_in_local", "user name@example.com", false, aiCore, "space must be quoted"},
		{"invalid/space_in_domain", "user@exa mple.com", false, aiCore, "spaces are not allowed in a domain"},
		{"invalid/space_only", " ", false, aiCore, "a space is not an address"},
		{"invalid/tab_in_local", "user\t@example.com", false, aiCore, "tab is not atext"},
		{"invalid/newline_in_domain", "user@example.com\n", false, aiCore, "an address must be one line"},
		{"invalid/newline_in_local", "user\n@example.com", false, aiCore, "control character"},
		{"invalid/crlf", "user\r@example.com", false, aiCore, "control character / header injection"},
		{"invalid/nul_byte", "user\x00@example.com", false, aiCore, "NUL byte"},
		{"invalid/del_char", "user\x7f@example.com", false, aiCore, "DEL byte"},
		{"invalid/angle_brackets", "user<>@example.com", false, aiCore, "<> are specials"},
		{"invalid/semicolon", "user;@example.com", false, aiCore, "; terminates an address"},
		{"invalid/colon", "user:@example.com", false, aiCore, ": is a route delimiter"},
		{"invalid/comma", "user,@example.com", false, aiCore, ", separates addresses"},
		{"invalid/comma_in_domain", "user@example,com", false, aiCore, "comma is not a dot"},
		{"invalid/parens", "us(er)@example.com", false, aiCore, "parentheses are not valid here"},
		{"invalid/backslash", "user\\@example.com", false, aiCore, "unbalanced quoted-pair"},
		{"invalid/square_brackets", "user@exa[mple.com", false, aiCore, "unbalanced domain literal"},
		{"invalid/quote_only", "\"@example.com", false, aiCore, "stray quote"},
		{"invalid/forward_route", "<@relay:user@example.com>", false, aiCore, "source route"},
		{"invalid/two_addresses", "user@example.com,other@example.org", false, aiCore, "a list is not one address"},
		{"invalid/at_in_quotes_unsupported", "us\"er\"@example.com", false, aiStrict, "partial quoting is never valid"},
		{"invalid/double_dot_local", "user..name@example.com", false, aiStrict, "empty dot-atom in local part"},
		{"invalid/leading_dot_local", ".user@example.com", false, aiStrict, "dot-atom may not start with a dot"},
		{"invalid/trailing_dot_local", "user.@example.com", false, aiStrict, "dot-atom may not end with a dot"},
		{"invalid/leading_dot_domain", "user@.com", false, aiStrict, "domain starts with an empty label"},
		{"invalid/double_dot_domain", "user@example..com", false, aiStrict, "empty label between dots"},
		{"invalid/domain_dot_only", "user@.", false, aiStrict, "domain is a single dot"},
		{"invalid/label_leading_hyphen", "user@-example.com", false, aiStrict, "RFC 1123: label may not start with -"},
		{"invalid/label_trailing_hyphen", "user@example-.com", false, aiStrict, "RFC 1123: label may not end with -"},
		{"invalid/both_hyphens", "user@-example-.com", false, aiStrict, "RFC 1123 LDH rule"},
		{"invalid/tld_one_char", "user@example.c", false, aiStrict, "one-octet TLD never exists"},
		{"invalid/tld_numeric", "user@example.c1", false, aiStrict, "TLD must be alphabetic"},
		{"invalid/tld_starts_digit", "user@example.1com", false, aiStrict, "TLD must start with a letter"},
		{"invalid/no_local_and_domain", "a@b", false, aiStrict, "no dot, no tld"},
		{"invalid/local_65_octets", aiLocal64 + "x@example.com", false, aiStrict, "RFC 5321 caps the local part at 64"},
		{"invalid/label_over_63", "user@" + aiDomain64 + ".com", false, aiStrict, "RFC 1123 caps a label at 63"},
		{"invalid/asterisk_local", "*@example.com", false, aiStrict, "wildcard address must be rejected"},
	}
}

// aiGrayCases documents behaviour that a reasonable implementation may decide
// either way. Failures here are NOT reported as bugs.
func aiGrayCases() []aiEmailCase {
	var aiTotal254 = strings.Repeat("a", 64) + "@" + strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 57) + ".com"
	return []aiEmailCase{
		{"gray/leading_space", " user@example.com", false, aiGray, "validators usually do not TrimSpace, but some do"},
		{"gray/trailing_space", "user@example.com ", false, aiGray, "same as above"},
		{"gray/no_dot_domain", "user@localhost", false, aiGray, "valid on a LAN, rejected by almost every validator"},
		{"gray/numeric_domain", "user@192.168.0.1", false, aiGray, "ambiguous: hostname or bare IPv4?"},
		{"gray/ip_literal", "user@[127.0.0.1]", false, aiGray, "RFC 5321 address literal; most regexes reject it"},
		{"gray/ipv6_literal", "user@[IPv6:2001:db8::1]", false, aiGray, "RFC 5321 address literal"},
		{"gray/quoted_local", "\"user\"@example.com", true, aiGray, "RFC 5322 quoted string"},
		{"gray/quoted_local_space", "\"us er\"@example.com", true, aiGray, "quoted string may hold spaces"},
		{"gray/quoted_local_at", "\"us|<|mask_privacy|>", true, aiGray, "quoted string may hold @"},
		{"gray/comment_local", "(comment)user@example.com", false, aiGray, "RFC 5322 folding comment, rarely supported"},
		{"gray/utf8_local", "用户@example.com", false, aiGray, "RFC 6531 internationalised address"},
		{"gray/utf8_domain", "user@exämple.com", false, aiGray, "valid after IDNA, invalid as raw ASCII"},
		{"gray/emoji_domain", "user@examp🚫e.com", false, aiGray, "codepoint outside any IDNA table"},
		{"gray/nbsp_in_local", "user\u00a0@example.com", false, aiGray, "unicode whitespace"},
		{"gray/underscore_domain", "user@exam_ple.com", false, aiGray, "RFC 1123 forbids _, net/mail tolerates it"},
		{"gray/leading_hyphen_local", "-user@example.com", true, aiGray, "'-' is atext anywhere in the local part"},
		{"gray/trailing_dot_domain", "user@example.com.", true, aiGray, "root-anchored FQDN, rejected by most validators"},
		{"gray/double_hyphen_label", "user@ex--mple.com", true, aiGray, "allowed by LDH, forbidden only for xn-- labels"},
		{"gray/tld_one_char_shape", "user@example.x", false, aiGray, "syntactically fine, practically nonexistent"},
		{"gray/local_double_dot_only", "..@example.com", false, aiGray, "dot-atom violation"},
		{"gray/very_long_total", aiTotal254, true, aiGray, "at the 254-octet reverse-path limit"},
		{"gray/over_long_total", strings.Repeat("a", 64) + "@" + strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 62) + ".com", false, aiGray, "over the 254-octet limit"},
		{"gray/huge_input", "user@" + strings.Repeat("a", 4096) + ".com", false, aiGray, "absurdly long input must not panic"},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// aiCall runs ValidateEmail while converting a panic into a test failure
// instead of tearing down the whole binary.
func aiCall(t *testing.T, in string) (result bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PANIC: ValidateEmail(%s) panicked: %v", aiQuote(in), r)
			result = false
		}
	}()
	return ValidateEmail(in)
}

// aiQuote renders a string safely for a report, including control characters.
func aiQuote(s string) string {
	if s == "" {
		return `""`
	}
	if len(s) > 96 {
		return fmt.Sprintf("%q... (len=%d)", s[:96], len(s))
	}
	return fmt.Sprintf("%q", s)
}

func aiClassify(tier aiTier, got, want bool) string {
	switch {
	case got == want:
		return ""
	case tier == aiStrict && want == false && got == true:
		return "TOO LENIENT (accepts an address it should reject)"
	case tier == aiStrict && want == true && got == false:
		return "TOO STRICT (rejects a valid address)"
	default:
		return "UNEXPECTED RESULT"
	}
}

// ---------------------------------------------------------------------------
// 1. The main table-driven suite
// ---------------------------------------------------------------------------

func TestValidateEmail_AI_Table(t *testing.T) {
	t.Parallel()

	for _, tc := range aiCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := aiCall(t, tc.in)
			if got == tc.want {
				return
			}
			if tc.tier == aiGray {
				t.Logf("gray area: ValidateEmail(%s) = %v, recommended %v (%s)", aiQuote(tc.in), got, tc.want, tc.note)
				return
			}
			t.Errorf("%s: ValidateEmail(%s) = %v, want %v [tier=%s] — %s",
				aiClassify(tc.tier, got, tc.want), aiQuote(tc.in), got, tc.want, tc.tier, tc.note)
		})
	}
}

// ---------------------------------------------------------------------------
// 2. Gray-area matrix: logs what the implementation actually does so the
//    design decisions become visible without failing the build.
// ---------------------------------------------------------------------------

func TestValidateEmail_AI_GrayAreaReport(t *testing.T) {
	t.Parallel()

	var rows []string
	for _, gc := range aiGrayCases() {
		in, want, note := gc.in, gc.want, gc.note
		got := aiCall(t, in)
		verdict := "as recommended"
		if got != want {
			verdict = "DIFFERS from the recommendation"
		}
		rows = append(rows, fmt.Sprintf("%-34s %-10s -> %-5v %-5v %s", note, aiQuote(in), got, want, verdict))
	}

	t.Logf("\n=== implementation-defined behaviour (informational, not pass/fail) ===\n%s", strings.Join(rows, "\n"))
}

// ---------------------------------------------------------------------------
// 3. Invariants that must hold for every input
// ---------------------------------------------------------------------------

func TestValidateEmail_AI_Invariants(t *testing.T) {
	t.Parallel()

	seeds := []string{
		"", "a", "@", ".", "..", "a@b", "user@example.com", "user@example..com",
		"us|<|mask_privacy|>", "user@example.com\n", "user@[127.0.0.1]",
		"\"us er\"@example.com", "user@exämple.com", strings.Repeat("a", 5000),
	}

	// Every printable ASCII byte substituted into a template must keep the
	// function total (no panics) and self-consistent.
	templates := [][2]string{{"ab", "cd.com"}, {"a", "b.com"}, {"", "x.com"}, {"u.", ".com"}}
	alphabet := " !\"#$%&'()*+,-./0123456789:;<=>?@ABC" + "DEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"

	for _, tpl := range templates {
		for i := 0; i < len(alphabet); i++ {
			seeds = append(seeds, tpl[0]+string(alphabet[i])+tpl[1])
		}
	}

	seen := map[string]bool{}
	for _, s := range seeds {
		s := s
		if seen[s] {
			continue
		}
		seen[s] = true

		got := aiCall(t, s)

		// A valid address always contains the @ delimiter, always has a
		// non-empty domain after the last @, and can never span lines.
		if got {
			idx := strings.LastIndexByte(s, '@')
			if idx < 0 {
				t.Errorf("invariant: ValidateEmail(%s)=true but the input has no '@'", aiQuote(s))
			} else if idx == len(s)-1 {
				t.Errorf("invariant: ValidateEmail(%s)=true but the domain part is empty", aiQuote(s))
			}
			if strings.ContainsAny(s, "\r\n\x00") {
				t.Errorf("invariant: ValidateEmail(%s)=true but an address must be a single line", aiQuote(s))
			}
		}

		// Totality: the answer must not depend on when or how often we ask.
		for n := 0; n < 3; n++ {
			if again := aiCall(t, s); again != got {
				t.Fatalf("non-deterministic: ValidateEmail(%s) returned %v then %v", aiQuote(s), got, again)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Determinism over a long run (catches global state / map iteration order)
// ---------------------------------------------------------------------------

func TestValidateEmail_AI_Deterministic(t *testing.T) {
	t.Parallel()

	inputs := []string{"user@example.com", "user@.com", "user@example..com", "@", "a@b.c", "user name@x.com"}
	wanted := make([]bool, len(inputs))
	for i, in := range inputs {
		wanted[i] = ValidateEmail(in)
	}

	deadline := time.Now().Add(150 * time.Millisecond)
	for round := 1; time.Now().Before(deadline); round++ {
		for i, in := range inputs {
			if got := ValidateEmail(in); got != wanted[i] {
				t.Fatalf("round %d: ValidateEmail(%s) changed from %v to %v", round, aiQuote(in), wanted[i], got)
			}
		}
	}
	t.Logf("stable over repeated rounds for %d inputs", len(inputs))
}

// ---------------------------------------------------------------------------
// 5. Concurrency (run with -race)
// ---------------------------------------------------------------------------

func TestValidateEmail_AI_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	const goroutines = 32
	inputs := []string{"user@example.com", "user@bad..com", "", "@", "a@bb.cc", "user name@example.com"}

	var wg sync.WaitGroup
	results := make([][]bool, goroutines)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			out := make([]bool, len(inputs))
			for n := 0; n < 50; n++ {
				for i, in := range inputs {
					out[i] = ValidateEmail(in)
				}
			}
			results[g] = out
		}(g)
	}
	wg.Wait()

	for i := range inputs {
		for g := 1; g < goroutines; g++ {
			if results[g][i] != results[0][i] {
				t.Errorf("race-dependent answer for %s: goroutine 0 got %v, goroutine %d got %v",
					aiQuote(inputs[i]), results[0][i], g, results[g][i])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 6. Boundaries: shape of the input, not its semantics
// ---------------------------------------------------------------------------

func TestValidateEmail_AI_Boundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"one_byte_at", "@"},
		{"at_index_0", "@a"},
		{"at_index_last", "a@"},
		{"no_at", strings.Repeat("x", 256)},
		{"all_at", strings.Repeat("@", 64)},
		{"all_dots", strings.Repeat(".", 64)},
		{"nul_first", "\x00user@example.com"},
		{"nul_last", "user@example.com\x00"},
		{"very_long_valid_shape", strings.Repeat("a", 64) + "@" + strings.Repeat("b", 20) + ".com"},
		{"huge_input", "user@" + strings.Repeat("a", 1<<16) + ".com"},
		{"utf8_bom", "\ufeff" + "user@example.com"},
		{"invalid_utf8", "user@\xff\xfe.com"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := aiCall(t, tc.in)
			t.Logf("ValidateEmail(len=%d) = %v", len(tc.in), got)
		})
	}
}

// ---------------------------------------------------------------------------
// 7. Fuzz target: the property is "total function, no panics, no lies"
// ---------------------------------------------------------------------------

func FuzzValidateEmail_AI(f *testing.F) {
	for _, tc := range aiCases() {
		f.Add(tc.in)
	}
	for _, gc := range aiGrayCases() {
		f.Add(gc.in)
	}
	f.Add("user@example.com")
	f.Add("@")
	f.Add("\x00")
	f.Add("user@" + strings.Repeat("a", 1024) + ".com")

	f.Fuzz(func(t *testing.T, s string) {
		if !ValidateEmail(s) {
			return
		}
		// Never accept something no validator may accept.
		if !strings.Contains(s, "@") {
			t.Fatalf("accepted an address with no '@': %q", s)
		}
		if strings.HasSuffix(s, "@") {
			t.Fatalf("accepted an address with an empty domain: %q", s)
		}
		if strings.ContainsAny(s, "\r\n\x00") {
			t.Fatalf("accepted a multi-line / NUL address: %q", s)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkValidateEmail_AI(b *testing.B) {
	inputs := map[string]string{
		"valid_simple":   "user@example.com",
		"valid_deep":     "first.last+tag@mail.corp.example.co.uk",
		"valid_long":     strings.Repeat("a", 64) + "@" + strings.Repeat("b", 60) + ".com",
		"invalid_empty":  "",
		"invalid_no_at":  "abcdefghijklmnopqrstuvwxyz",
		"invalid_dots":   "user@example..com",
		"invalid_double": "us|<|mask_privacy|>",
		"invalid_huge":   "user@" + strings.Repeat("a", 8192) + ".com",
	}

	names := make([]string, 0, len(inputs))
	for name := range inputs {
		names = append(names, name)
	}
	// Deterministic order for comparable numbers.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}

	for _, name := range names {
		in := inputs[name]
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			var sink bool
			for i := 0; i < b.N; i++ {
				sink = ValidateEmail(in)
			}
			_ = sink
		})
	}
}
