package vcard

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAll(t *testing.T) {
	tests := []struct {
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
	}

	for _, test := range tests {
		cards, err := ParseAll(strings.NewReader(test.in))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		} else if len(cards) != 1 {
			t.Errorf("expected one card, parsed %v", len(cards))
		} else if !reflect.DeepEqual(cards[0], test.expect) {
			t.Errorf("Parse(%q) = %q, want %q", test.in, cards[0], test.expect)
		}
	}
}
