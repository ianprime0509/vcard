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
		{"tabs\n\t are important", "tabs are important"},
		{"but\r\n\t they can be annoying", "but they can be annoying"},
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
			t.Errorf("unfolding %q: got %q, want %q", test.in, sb.String(), test.out)
		}
	}
}

func TestPeekByte(t *testing.T) {
	r := NewUnfoldingReader(strings.NewReader("h\n i"))
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != 'h' {
		t.Fatalf("read %q, want %q", b, 'h')
	}
	b, err = r.PeekByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != 'i' {
		t.Fatalf("peeked %q, want %q", b, 'i')
	}
	b, err = r.ReadByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != 'i' {
		t.Fatalf("read %q, want %q", b, 'i')
	}
	_, err = r.PeekByte()
	if err != io.EOF {
		t.Fatal("expected EOF on peek")
	}
	_, err = r.ReadByte()
	if err != io.EOF {
		t.Fatal("expected EOF on read")
	}
}

func TestFold(t *testing.T) {
	const width = 10
	tests := []struct {
		in  string
		out string
	}{
		{"nothing", "nothing"},
		{"BEGIN:VCARD", "BEGIN:VC\r\n ARD"},
		{"I am a rather long string", "I am a r\r\n ather l\r\n ong str\r\n ing"},
		{"new\nline", "new\r\nline"},
		{"new\r\nline", "new\r\nline"},
		{"many\n\r\nlines", "many\r\n\r\nlines"},
		{"carriage\rreturn", "carriage\r\n \rreturn"},
		{"tricky!!\r\n", "tricky!!\r\n"},
		{"tricky!!\n", "tricky!!\r\n"},
		{"tricky!!!\n", "tricky!!\r\n !\r\n"},
		{"こんにちは世界", "こん\r\n にち\r\n は世\r\n 界"},
		{"return\r", "return\r"},
		{"returns\r\r\r", "returns\r\r\n \r\r"},
		{"lines\n\n\r\nfor you", "lines\r\n\r\n\r\nfor you"},
		{"bad\n input", "bad\r\n input"},
		{"do\r\n not", "do\r\n not"},
		{"はい!!\r\n", "はい!!\r\n"},
		{"はい!!\n", "はい!!\r\n"},
	}

	for _, test := range tests {
		folded := Fold(test.in, width)
		if folded != test.out {
			t.Errorf("Fold(%q) = %q, want %q", test.in, folded, test.out)
		}
	}
}
