package vcard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Card is a container for vCard data, mapping each property name to a slice
// containing the details of each occurrence of the property in the order they
// appeared in the input.
type Card map[string][]Property

// Property is a container for the information stored in a vCard property,
// except for the name.
type Property struct {
	Group  string
	Params map[string][]string
	Values []string
}

// Parse parses as many vCards from the given input as possible, until EOF is
// reached or a parsing error occurs. If parsing fails at any point, the
// returned slice will contain any cards that were successfully parsed
// before the error.
func Parse(r io.Reader) ([]Card, error) {
	var cards []Card
	p := parser{r: NewUnfoldingReader(bufio.NewReader(r))}

	for _, err := p.r.PeekByte(); err != io.EOF; _, err = p.r.PeekByte() {
		card, err := p.parseCard()
		if err != nil {
			return cards, fmt.Errorf("on line %v: %v", p.r.Line(), err)
		}
		cards = append(cards, card)

	}

	return cards, nil
}

// parser contains the state of the parse operation performed by Parse.
type parser struct {
	r *UnfoldingReader
}

// parseCard parses a complete vCard from the input.
func (p *parser) parseCard() (Card, error) {
	card := make(Card)
	name, prop, err := p.parseProperty()
	if err != nil {
		return Card{}, err
	} else if name != "BEGIN" || len(prop.Group) != 0 || len(prop.Params) != 0 ||
		len(prop.Values) != 1 || strings.ToUpper(prop.Values[0]) != "VCARD" {
		return Card{}, errors.New("expected beginning of card")
	}

	for name, prop, err = p.parseProperty(); err == nil; name, prop, err = p.parseProperty() {
		if name == "END" {
			if len(prop.Group) != 0 || len(prop.Params) != 0 ||
				len(prop.Values) != 1 || strings.ToUpper(prop.Values[0]) != "VCARD" {
				return Card{}, errors.New("malformed end tag")
			}
			return card, nil
		}
		card[name] = append(card[name], prop)
	}

	if err == io.EOF {
		return Card{}, errors.New("unexpected end of input before ending card")
	}
	return Card{}, err
}

// parseProperty parses a single property
func (p *parser) parseProperty() (name string, prop Property, err error) {
	// Parse name (or group).
	nm, err := p.parseName("expected property name")
	if err != nil {
		return "", Property{}, err
	}

	b, err := p.demandByte("expected parameters or property value")
	if err != nil {
		return "", Property{}, err
	}
	// If we parsed the group, now parse the name.
	if b == '.' {
		prop.Group = nm
		nm, err = p.parseName("expected property name")
		if err != nil {
			return "", Property{}, err
		}
		b, err = p.demandByte("expected parameters or property value")
	}
	name = nm

	if err != nil {
		return "", Property{}, err
	}
	if b == ';' {
		// Parse any parameters.
		params, err := p.parseParameters()
		if err != nil {
			return "", Property{}, err
		}
		prop.Params = params
		b, err = p.demandByte("expected property value")
	}

	if err != nil {
		return "", Property{}, err
	}
	if b != ':' {
		fmt.Printf("%q\n", b)
		return "", Property{}, errors.New("expected ':'")
	}

	values, err := p.parsePropertyValues()
	if err != nil {
		return "", Property{}, err
	}
	prop.Values = values

	b, err = p.r.ReadByte()
	if err == io.EOF {
		return name, prop, nil
	} else if err != nil {
		return "", Property{}, err
	}
	if b != '\n' {
		return "", Property{}, fmt.Errorf("unexpected character %q after property value", b)
	}
	return name, prop, nil
}

// parsePropertyValues parses several property values, separated by commas.
func (p *parser) parsePropertyValues() ([]string, error) {
	var values []string

	var value string
	var err error
	for value, err = p.parsePropertyValue(); err == nil; value, err = p.parsePropertyValue() {
		values = append(values, value)
		b, err := p.r.PeekByte()
		if err == io.EOF {
			return values, nil
		} else if err != nil {
			return nil, err
		} else if b != ',' {
			return values, nil
		}
		p.r.ReadByte()
	}
	return nil, err
}

// parsePropertyValue parses a single property value. Since a property value
// may be empty, the returned error may be nil even if the returned string
// is empty.
func (p *parser) parsePropertyValue() (string, error) {
	var bs []byte

	var b byte
	var err error
	for b, err = p.r.PeekByte(); err == nil; b, err = p.r.PeekByte() {
		if !isValueChar(b) {
			return string(bs), nil
		}
		p.r.ReadByte()
		if b == '\\' {
			b2, err := p.demandByte("expected escaped character")
			if err != nil {
				return "", err
			}
			if b2 == ',' || b2 == '\\' {
				bs = append(bs, b2)
			} else if b2 == ';' {
				bs = append(bs, '\\', ';')
			} else {
				return "", fmt.Errorf("%q cannot be escaped", b2)
			}
		} else {
			bs = append(bs, b)
		}
	}
	if err == io.EOF {
		return string(bs), nil
	}
	return "", err
}

// isValueChar returns whether the given byte may be present in a property
// value.
func isValueChar(b byte) bool {
	return b == '\t' || (' ' <= b && b != ',')
}

// parseParameters parses a set of property parameters. Since parameters are
// optional, both the map and error returned from this method may be nil.
func (p *parser) parseParameters() (map[string][]string, error) {
	params := make(map[string][]string)

	var key string
	var values []string
	var err error
	for key, values, err = p.parseParameter(); err == nil; key, values, err = p.parseParameter() {
		if _, ok := params[key]; ok {
			return nil, fmt.Errorf("duplicate parameter %q", key)
		}
		params[key] = values

		b, err := p.r.PeekByte()
		if err == io.EOF {
			return params, nil
		} else if err != nil {
			return nil, err
		} else if b != ';' {
			return params, nil
		}
		p.r.ReadByte()
	}
	return nil, err
}

// parseParameter parses a single property parameter. If the returned error
// is nil, then the key and values will both be non-nil.
func (p *parser) parseParameter() (key string, values []string, err error) {
	key, err = p.parseName("expected parameter name")
	if err != nil {
		return "", nil, err
	}

	msg := fmt.Sprintf("expected '=' after parameter name %v", key)
	b, err := p.demandByte(msg)
	if err != nil {
		return "", nil, err
	} else if b != '=' {
		return "", nil, errors.New(msg)
	}

	var value string
	for value, err = p.parseParameterValue(); err == nil; value, err = p.parseParameterValue() {
		values = append(values, value)
		b, err := p.r.PeekByte()
		if err == io.EOF {
			return key, values, nil
		} else if err != nil {
			return "", nil, err
		} else if b != ',' {
			return key, values, nil
		}
		p.r.ReadByte()
	}
	return "", nil, err
}

// parseParameterValue parses a single property parameter value. The returned
// string may be empty even if the error is non-nil, since parameter values
// may be empty.
func (p *parser) parseParameterValue() (string, error) {
	b, err := p.r.PeekByte()
	if err == io.EOF {
		return "", nil
	} else if err != nil {
		return "", err
	}

	if b == '"' {
		p.r.ReadByte()
		return p.parseQuotedParameterValue()
	}
	return p.parseUnquotedParameterValue()
}

// parseQuotedParameterValue parses the inner part of a paramter enclosed in
// double quotes. It will also consume the closing quote.
func (p *parser) parseQuotedParameterValue() (string, error) {
	var bs []byte

	var b byte
	var err error
	for b, err = p.r.ReadByte(); err == nil; b, err = p.r.ReadByte() {
		if b == '"' {
			return string(bs), nil
		} else if !isQuoteSafeChar(b) {
			return "", fmt.Errorf("unexpected byte %q in quoted parameter value", b)
		}
		bs = append(bs, b)
	}

	if err != nil && err != io.EOF {
		return "", err
	}
	return "", errors.New("unexpected end of quoted parameter value")
}

// isQuoteSafeChar returns whether the given byte may appear within a quoted
// parameter value.
func isQuoteSafeChar(b byte) bool {
	return b == ' ' || b == '\t' || b == '!' || '"' < b
}

// parseUnquotedParameterValue parses a parameter value not enclosed in double
// quotes.
func (p *parser) parseUnquotedParameterValue() (string, error) {
	var bs []byte

	var b byte
	var err error
	for b, err = p.r.PeekByte(); err == nil; b, err = p.r.PeekByte() {
		if !isSafeChar(b) {
			return string(bs), nil
		}
		p.r.ReadByte()
		bs = append(bs, b)
	}

	if err != nil {
		return string(bs), err
	}
	return string(bs), nil
}

// isSafeChar returns whether the given byte may appear within an unquoted
// parameter value.
func isSafeChar(b byte) bool {
	// Note: the official RFC for some reason includes ',' as a safe
	// character; this is probably an oversight.
	return b == ' ' || b == '\t' || b == '!' || ('"' < b && b != ';' && b != ':' && b != ',')
}

// parseName parses anything that has the format of a property name, group
// or parameter name. If the parsed name is empty but no other error occurred,
// an error will be returned wrapping the given string.
func (p *parser) parseName(missing string) (string, error) {
	var bs []byte

	var b byte
	var err error
	for b, err = p.r.PeekByte(); err == nil; b, err = p.r.PeekByte() {
		if ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') || b == '-' {
			bs = append(bs, b)
		} else if 'a' <= b && b <= 'z' {
			// Convert b to uppercase.
			bs = append(bs, b+'A'-'a')
		} else {
			break
		}
		p.r.ReadByte()
	}

	if err != nil {
		return string(bs), err
	} else if len(bs) == 0 {
		return "", errors.New(missing)
	}
	return string(bs), nil
}

// demandByte reads the next byte according to readByte, but converts an EOF
// error into an error wrapping the given string.
func (p *parser) demandByte(missing string) (byte, error) {
	b, err := p.r.ReadByte()
	if err == io.EOF {
		return 0, errors.New(missing)
	}
	return b, err
}
