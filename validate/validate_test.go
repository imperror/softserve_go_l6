// Homework — Task 2: extend emailCases and/or phoneCases below to at
// least 8 cases each (your mentor may ask for just one of the two
// functions), then implement validate.go until every subtest passes.
//
// Like todo_test.go, this file uses t.Run per case and t.Errorf (not
// t.Fatalf) for the actual assertions, so one wrong case never hides
// the others. Run `go test -v ./validate/...` and read every FAIL line.
package validate

import "testing"

const minCases = 8

// emailCases is the table of test cases for ValidateEmail.
//
// TODO: add at least 5 more cases here — for example: whitespace inside
// the address, a missing domain, a trailing dot, consecutive dots, a
// very long local part, or a unicode character.
var emailCases = []struct {
	name  string
	input string
	want  bool
}{
	{"valid simple", "student@softserve.academy", true},
	{"missing at sign", "student-softserve.academy", false},
	{"empty string", "", false},

	// TODO: add at least 5 more cases here.
	{"whitespace inside", "stu dent@softserve.academy", false},
	{"whitespace trailing", "student@softserve.academy ", false},
	{"multiple at signs", "stu@dent@softserve.academy", false},
	{"missing local part", "@softserve.academy", false},
	{"missing domain part", "student@", false},
	{"no dot in domain", "student@localhost", false},
	{"trailing dot in domain", "student@softserve.academy.", false},
	{"consecutive dots in local", "stud..ent@softserve.academy", false},
	{"leading dot in local", ".student@softserve.academy", false},
	{"plus subaddress", "student+hw@softserve.academy", true},
	{"hyphenated domain label", "student@soft-serve.academy", true},
	{"single letter tld", "student@softserve.a", false},
	{"numeric tld", "student@softserve.123", false},
	{"unicode local part", "юзер@softserve.academy", false},
	{"uppercase ascii allowed", "Student@SoftServe.Academy", true},
}

func TestValidateEmail(t *testing.T) {
	if len(emailCases) < minCases {
		t.Fatalf(
			"emailCases has %d case(s), need at least %d — add more edge cases to validate_test.go before this test can pass",
			len(emailCases), minCases,
		)
	}

	for _, tc := range emailCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateEmail(tc.input)
			if got != tc.want {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// phoneCases is the table of test cases for ValidatePhone.
// This is only required if your mentor asked you to validate phone
// numbers instead of (or in addition to) email addresses.
//
// TODO: add at least 5 more cases here — for example: missing digits,
// letters mixed in, an unexpected country code format, or extra
// separators like spaces, dots or parentheses.
var phoneCases = []struct {
	name  string
	input string
	want  bool
}{
	{"valid with plus", "+380501234567", true},
	{"contains letters", "050-abc-4567", false},
	{"empty string", "", false},

	// TODO: add at least 5 more cases here.
	{"dashed national format", "050-123-4567", true},
	{"spaces and parentheses", "(050) 123 4567", true},
	{"dotted format", "380.50.123.4567", true},
	{"too few digits", "12345", false},
	{"too many digits", "+1234567890123456", false},
	{"no digits at all", "---", false},
	{"letters only", "abcdefghij", false},
	{"plus sign not at start", "380+501234567", false},
	{"double plus", "++380501234567", false},
	{"underscore separator", "050_123_4567", false},
	{"tab character", "\t", false},
	{"non ascii digits", "050١23٤567", false},
	{"local number without country code", "0501234567", true},
}

func TestValidatePhone(t *testing.T) {
	if len(phoneCases) < minCases {
		t.Fatalf(
			"phoneCases has %d case(s), need at least %d — add more edge cases to validate_test.go before this test can pass",
			len(phoneCases), minCases,
		)
	}

	for _, tc := range phoneCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ValidatePhone(tc.input)
			if got != tc.want {
				t.Errorf("ValidatePhone(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
