package vcard

import (
	"io"
	"reflect"
	"strings"
	"testing"
)

// A sample vCard from Wikipedia: https://en.wikipedia.org/wiki/VCard#vCard_3.0
const sampleVCard = `BEGIN:VCARD
VERSION:3.0
N:Gump;Forrest;;Mr.;
FN:Forrest Gump
ORG:Bubba Gump Shrimp Co.
TITLE:Shrimp Man
PHOTO;VALUE=URI;TYPE=GIF:http://www.example.com/dir_photos/my_photo.gif
TEL;TYPE=WORK,VOICE:(111) 555-1212
TEL;TYPE=HOME,VOICE:(404) 555-1212
ADR;TYPE=WORK,PREF:;;100 Waters Edge;Baytown;LA;30314;United States of America
LABEL;TYPE=WORK,PREF:100 Waters Edge\nBaytown\, LA 30314\nUnited States of America
ADR;TYPE=HOME:;;42 Plantation St.;Baytown;LA;30314;United States of America
LABEL;TYPE=HOME:42 Plantation St.\nBaytown\, LA 30314\nUnited States of America
EMAIL:forrestgump@example.com
REV:2008-04-24T19:52:43Z
END:VCARD`

var sampleVCardParsed = &Card{map[string][]Property{
	"VERSION": {{values: []string{"3.0"}}},
	"N":       {{values: []string{"Gump;Forrest;;Mr.;"}}},
	"FN":      {{values: []string{"Forrest Gump"}}},
	"ORG":     {{values: []string{"Bubba Gump Shrimp Co."}}},
	"TITLE":   {{values: []string{"Shrimp Man"}}},
	"PHOTO": {{
		params: map[string][]string{
			"VALUE": {"URI"},
			"TYPE":  {"GIF"},
		},
		values: []string{"http://www.example.com/dir_photos/my_photo.gif"},
	}},
	"TEL": {{
		params: map[string][]string{
			"TYPE": {"WORK", "VOICE"},
		},
		values: []string{"(111) 555-1212"},
	}, {
		params: map[string][]string{
			"TYPE": {"HOME", "VOICE"},
		},
		values: []string{"(404) 555-1212"},
	}},
	"ADR": {{
		params: map[string][]string{
			"TYPE": {"WORK", "PREF"},
		},
		values: []string{
			";;100 Waters Edge;Baytown;LA;30314;United States of America",
		},
	}, {
		params: map[string][]string{
			"TYPE": {"HOME"},
		},
		values: []string{
			";;42 Plantation St.;Baytown;LA;30314;United States of America",
		},
	}},
	"LABEL": {{
		params: map[string][]string{
			"TYPE": {"WORK", "PREF"},
		},
		values: []string{
			"100 Waters Edge\nBaytown, LA 30314\nUnited States of America",
		},
	}, {
		params: map[string][]string{
			"TYPE": {"HOME"},
		},
		values: []string{
			"42 Plantation St.\nBaytown, LA 30314\nUnited States of America",
		},
	}},
	"EMAIL": {{
		values: []string{"forrestgump@example.com"},
	}},
	"REV": {{
		values: []string{"2008-04-24T19:52:43Z"},
	}},
}}

func BenchmarkUnfoldedString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sampleVCardParsed.UnfoldedString()
	}
}

func BenchmarkParseAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseAll(strings.NewReader(sampleVCard))
	}
}

func TestUnfoldedString(t *testing.T) {
	// We need to trim the final newline (if any) so it doesn't show up as
	// a property in the list of lines.
	expectedLines := strings.Split(strings.TrimRight(sampleVCard, "\r\n"), "\n")
	unfolded := sampleVCardParsed.UnfoldedString()
	actualLines := strings.Split(strings.TrimRight(unfolded, "\r\n"), "\n")

	if len(actualLines) != len(expectedLines) {
		t.Fatalf("got %v lines, want %v", len(actualLines), len(expectedLines))
	}

	// The BEGIN, VERSION and END properties should be the same.
	if actualLines[0] != expectedLines[0] {
		t.Fatalf("got begin %q, want %q", actualLines[0], expectedLines[0])
	}
	if actualLines[1] != expectedLines[1] {
		t.Fatalf("got version %q, want %q", actualLines[1], expectedLines[1])
	}
	if actualLines[len(actualLines)-1] != expectedLines[len(expectedLines)-1] {
		t.Fatalf("got end %q, want %q", actualLines[len(actualLines)-1], expectedLines[len(expectedLines)-1])
	}

	// For the rest, the order doesn't matter. We checked equal lengths
	// above, so this is sufficient to check full equality.
	for _, line := range expectedLines {
		for _, actualLine := range actualLines {
			if line == actualLine {
				goto found
			}
		}
		t.Fatalf("wanted line %q but not found", line)
	found:
	}
}

var successTests = []struct {
	in     string
	expect *Card
}{
	{
		"BEGIN:VCARD\r\nPROP:value\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"BEG\r\n IN\r\n :VCARD\r\nPROP:va\r\n lue\r\nEND:\r\n VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP:value\r\nEND:VCARD",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"begin:vCard\r\nprop:value\r\nend:vCard\r\n",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"begin:vcard\nprop:value\nend:vcard\n",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"begin:vcard\npr\n op:\n value\nend:vcard\n",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"begin:vcard\nprop:value\nend:vcard",
		&Card{map[string][]Property{
			"PROP": {{values: []string{"value"}}},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP:value\r\nPROP:value2\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {
				{values: []string{"value"}},
				{values: []string{"value2"}},
			},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP-1;PARAM=test:value\r\nprop-2;param=\"test\":value2\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP-1": {{
				params: map[string][]string{"PARAM": {"test"}},
				values: []string{"value"},
			}},
			"PROP-2": {{
				params: map[string][]string{"PARAM": {"test"}},
				values: []string{"value2"},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nX-PROP;PARAM=test;PARAM2=test2,\"hello,there\":value\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"X-PROP": {{
				params: map[string][]string{
					"PARAM":  {"test"},
					"PARAM2": {"test2", "hello,there"},
				},
				values: []string{"value"},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP:value1,value2\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{
				values: []string{"value1", "value2"},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP:value1\\,value2\\\\,\\\\,\\;;\\;\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{
				values: []string{"value1,value2\\", "\\", "\\;;\\;"},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP:\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{
				values: []string{""},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nPROP:multiple\\nlines\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{
				values: []string{"multiple\nlines"},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nGROUP.PROP:value\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{
				group:  "GROUP",
				values: []string{"value"},
			}},
		}},
	},
	{
		"BEGIN:VCARD\r\nGroup.Prop:value\r\nEND:VCARD\r\n",
		&Card{map[string][]Property{
			"PROP": {{
				group:  "GROUP",
				values: []string{"value"},
			}},
		}},
	},
	{sampleVCard, sampleVCardParsed},
}

func TestParseAll(t *testing.T) {
	for _, test := range successTests {
		cards, err := ParseAll(strings.NewReader(test.in))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		} else if len(cards) != 1 {
			t.Errorf("expected one card, parsed %v", len(cards))
		} else if !reflect.DeepEqual(cards[0], test.expect) {
			t.Errorf("ParseAll(%q)[0] = %q, want %q", test.in, cards[0], test.expect)
		}
	}
}

func TestParse(t *testing.T) {
	for _, test := range successTests {
		p := NewParser(strings.NewReader(test.in))
		card, err := p.Next()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		} else if !reflect.DeepEqual(card, test.expect) {
			t.Errorf("Parse(%q) = %q, want %q", test.in, card, test.expect)
		}
		card, err = p.Next()
		if err == nil {
			t.Errorf("parsed card after end of input: %q", card)
		} else if err != io.EOF {
			t.Errorf("unexpected error at end of input: %v", err)
		}
	}
}

var failureTests = []struct {
	in   string
	line int
	msg  string
}{
	{"PROP:VALUE\r\nEND:VCARD", 1, "expected beginning of card"},
	{"BEGIN:VCARD\r\n", 2, "unexpected end of input"},
	{"BEGIN:VCARD\r\nEND:SOMETHING\r\n", 2, "malformed end tag"},
	{" BAD\r\n", 1, "expected property name"},
	{"BEGIN:VCARD\r\nPROP\r\nEND:VCARD\r\n", 2, "expected ':'"},
	{"BEGIN:VCARD\r\nPROP=2\r\nEND:VCARD\r\n", 2, "expected ':'"},
	{"BEGIN:VCARD\r\nPROP;:2\r\nEND:VCARD\r\n", 2, "expected parameter name"},
	{"BEGIN:VCARD\r\nPROP;PARAM:2\r\nEND:VCARD\r\n", 2, "expected '=' after parameter name"},
	{"BEGIN:VCARD\r\nPROP;PARAM=\"test\n\":2\r\nEND:VCARD\r\n", 2, "unexpected byte '\\n' in quoted parameter value"},
	{"BEGIN:VCARD\r\nPROP:escape\\:\r\nEND:VCARD\r\n", 2, "':' cannot be escaped"},
}

func TestParseAllFailure(t *testing.T) {
	for _, test := range failureTests {
		cards, err := ParseAll(strings.NewReader(test.in))
		if err == nil {
			t.Errorf("successfully parsed %q", cards)
			continue
		}
		perr, ok := err.(ParseError)
		if !ok {
			t.Errorf("ParseAll(%q) error %q, not a parse error", test.in, err)
		}
		if test.line != perr.Line || !strings.Contains(perr.Message(), test.msg) {
			t.Errorf("ParseAll(%q) error %q, want %q on line %v", test.in, perr, test.msg, test.line)
		}
	}
}
