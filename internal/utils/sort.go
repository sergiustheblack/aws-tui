package utils

import (
	"strconv"
	"strings"
	"unicode"
)

// CompareValues compares two table cell values for sorting. Empty values sort
// after non-empty ones, values that are both plain numbers are compared
// numerically, and everything else uses a case-insensitive natural order so
// that e.g. "10.0.0.9" < "10.0.0.10" and "t3.2xlarge" < "t3.12xlarge".
func CompareValues(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	if fa, err := strconv.ParseFloat(a, 64); err == nil {
		if fb, err := strconv.ParseFloat(b, 64); err == nil {
			switch {
			case fa < fb:
				return -1
			case fa > fb:
				return 1
			}
		}
	}
	if c := compareNatural(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func compareNatural(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			si := i
			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}
			sj := j
			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}
			na := strings.TrimLeft(string(ra[si:i]), "0")
			nb := strings.TrimLeft(string(rb[sj:j]), "0")
			if len(na) != len(nb) {
				if len(na) < len(nb) {
					return -1
				}
				return 1
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			continue
		}
		if ra[i] != rb[j] {
			if ra[i] < rb[j] {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(ra)-i < len(rb)-j:
		return -1
	case len(ra)-i > len(rb)-j:
		return 1
	}
	return 0
}
