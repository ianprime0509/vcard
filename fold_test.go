package vcard

import (
	"io"
	"strings"
	"testing"
)

func TestUnfold(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"test", "test"},
		{"te\n st", "test"},
		{"te\nst", "te\nst"},
		{"te\r\n st", "test"},
		{"te\r\nst", "te\nst"},
		{"te\n\r\nst", "te\n\nst"},
		{"more\n  lines\r\n  here", "more lines here"},
		{"trailing\n", "trailing\n"},
		{"trailing\r\n", "trailing\n"},
		{"carriage\rreturn", "carriage\rreturn"},
		{"double\n \r\n line", "doubleline"},
		{"confuse\n\r\n me", "confuse\nme"},
		{"confused\n\r \n yet?", "confused\n\r yet?"},
		{"\nleading", "\nleading"},
		{"\r\nleading", "\nleading"},
		{" nothing to continue!", " nothing to continue!"},
		{"lots\n \n \n \n \r\n \r\n  of stuff", "lots of stuff"},
		{"こんにちは世界", "こんにちは世界"},
		{"こん\nにちは世界", "こん\nにちは世界"},
		{"こん\r\nにちは世界", "こん\nにちは世界"},
		{"こん\r\nにちは\n 世界", "こん\nにちは世界"},
		{"こん\r\nにちは\r\n 世界", "こん\nにちは世界"},
	}

	sb := new(strings.Builder)
	for _, test := range tests {
		sb.Reset()
		io.Copy(sb, NewUnfoldingReader(strings.NewReader(test.in)))
		if sb.String() != test.out {
			t.Errorf("unfolding %q: got %q, wanted %q", test.in, sb.String(), test.out)
		}
	}
}
