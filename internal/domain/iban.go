package domain

// ParseIBAN validates and normalizes an IBAN: a two-letter ISO 3166-1 country code, two check
// digits, and up to 30 further alphanumeric characters, with a correct ISO 7064 MOD 97-10
// checksum. It returns the IBAN uppercased and stripped of whitespace.
func ParseIBAN(s string) (string, error) {
	iban := normalizeIBAN(s)
	if iban == "" {
		return "", missingField("iban")
	}
	if !validIBANFormat(iban) || ibanChecksum(iban) != 1 {
		return "", invalidValue("iban", s)
	}
	return iban, nil
}

// validIBANFormat reports whether iban has the general IBAN shape: a two-letter country code,
// two check digits, and 11 to 30 further alphanumeric characters (15 to 34 overall, the range of
// every issued IBAN length).
func validIBANFormat(iban string) bool {
	if len(iban) < 15 || len(iban) > 34 {
		return false
	}
	for i, r := range iban {
		switch {
		case i < 2:
			if r < 'A' || r > 'Z' {
				return false
			}
		case i < 4:
			if r < '0' || r > '9' {
				return false
			}
		default:
			if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
				return false
			}
		}
	}
	return true
}

// ibanChecksum computes the ISO 7064 MOD 97-10 remainder of iban: the country code and check
// digits are moved to the end, letters are expanded to two digits each (A=10 ... Z=35), and the
// resulting number is reduced modulo 97 digit by digit to avoid overflow. A valid IBAN has a
// remainder of 1.
func ibanChecksum(iban string) int {
	rearranged := iban[4:] + iban[:4]
	remainder := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			remainder = (remainder*10 + int(r-'0')) % 97
		case r >= 'A' && r <= 'Z':
			remainder = (remainder*100 + int(r-'A') + 10) % 97
		}
	}
	return remainder
}
