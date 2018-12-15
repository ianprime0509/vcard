// Copyright 2018 Ian Johnson
//
// This file is part of vcard. Vcard is free software: you are free to use it
// for any purpose, make modified versions and share it with others, subject
// to the terms of the Apache license (version 2.0), a copy of which is
// provided alongside this project.

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
type Card struct {
	m map[string][]Property
}

// Get returns the properties corresponding to the given (case-insensitive)
// property name.
func (c *Card) Get(name string) []Property {
	return c.m[strings.ToUpper(name)]
}

// Add adds a property to the card.
func (c *Card) Add(name string, prop Property) {
	if c.m == nil {
		c.m = make(map[string][]Property)
	}
	name = strings.ToUpper(name)
	c.m[name] = append(c.m[name], prop)
}

// String returns the card in vCard syntax, properly folded such that each line
// fits within 77 bytes.
func (c *Card) String() string {
	return Fold(c.UnfoldedString(), 77)
}

// UnfoldedString returns the card in vCard syntax, but without folding any
// lines or using the "\r\n" line ending (it just uses the normal '\n').
func (c *Card) UnfoldedString() string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, "BEGIN:VCARD")
	for name, props := range c.m {
		for _, prop := range props {
			if len(prop.group) > 1 {
				fmt.Fprintf(sb, "%v.", prop.group)
			}
			sb.WriteString(name)
			for key, values := range prop.params {
				sb.WriteRune(';')
				writeParam(sb, key, values)
			}
			sb.WriteRune(':')
			writeValues(sb, prop.values)
			sb.WriteRune('\n')
		}
	}
	fmt.Fprintln(sb, "END:VCARD")
	return sb.String()
}

// Property is a container for the information stored in a vCard property,
// except for the name.
type Property struct {
	group  string
	params map[string][]string
	values []string
}

// Group returns the group of the property.
func (p *Property) Group() string {
	return p.group
}

// SetGroup sets the group of the property.
func (p *Property) SetGroup(group string) {
	p.group = strings.ToUpper(group)
}

// Param returns the values of a property parameter. Changes to the returned
// slice will be reflected in the property.
func (p *Property) Param(param string) []string {
	return p.params[strings.ToUpper(param)]
}

// SetParam sets the values of a property parameter. Further changes to the
// given slice will be reflected in the property (this method does not make
// a copy).
func (p *Property) SetParam(param string, values ...string) {
	if p.params == nil {
		p.params = make(map[string][]string)
	}
	p.params[strings.ToUpper(param)] = values
}

// Values returns the values of a property. Changes to the returned slice will
// be reflected in the property.
func (p *Property) Values() []string {
	return p.values
}

// SetValues sets the values of a property. Further changes to the given slice
// will be reflected in the property (this method does not make a copy).
func (p *Property) SetValues(values ...string) {
	p.values = values
}

// writeParam writes a parameter to the given Writer.
func writeParam(w io.Writer, key string, values []string) {
	fmt.Fprintf(w, "%v=", key)
	for i, value := range values {
		if i != 0 {
			fmt.Fprint(w, ';')
		}
		if strings.ContainsAny(value, ";:") {
			fmt.Fprintf(w, `"%v"`, value)
		} else {
			fmt.Fprint(w, value)
		}
	}
}

// writeValue writes a series of property values, separated by commas, to
// the given Writer.
func writeValues(w io.Writer, values []string) {
	for i, value := range values {
		if i != 0 {
			fmt.Fprint(w, ",")
		}
		writeValue(w, value)
	}
}

// writeValue writes a property value to the given Writer, taking care of
// escaping special characters.
func writeValue(w io.Writer, value string) {
	last := rune(-1)
	for _, r := range value {
		if last != -1 {
			// We shouldn't escape the backslash before a semicolon.
			if last == '\\' && r != ';' {
				fmt.Fprint(w, `\\`)
			} else if last == ',' {
				fmt.Fprint(w, `\,`)
			} else if last == '\n' {
				fmt.Fprint(w, `\n`)
			} else {
				fmt.Fprintf(w, "%c", last)
			}
		}
		last = r
	}
	if last == '\\' {
		fmt.Fprint(w, `\\`)
	} else if last == ',' {
		fmt.Fprint(w, `\,`)
	} else if last == '\n' {
		fmt.Fprint(w, `\n`)
	} else {
		fmt.Fprintf(w, "%c", last)
	}
}

// ParseAll parses as many vCards from the given input as possible, until EOF
// is reached or a parsing error occurs. If parsing fails at any point, the
// returned slice will contain any cards that were successfully parsed
// before the error.
func ParseAll(r io.Reader) ([]*Card, error) {
	var cards []*Card
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
func (p *parser) parseCard() (*Card, error) {
	card := &Card{m: make(map[string][]Property)}
	name, prop, err := p.parseProperty()
	if err != nil {
		return &Card{}, err
	} else if name != "BEGIN" || len(prop.group) != 0 || len(prop.params) != 0 ||
		len(prop.values) != 1 || strings.ToUpper(prop.values[0]) != "VCARD" {
		return &Card{}, errors.New("expected beginning of card")
	}

	for name, prop, err = p.parseProperty(); err == nil; name, prop, err = p.parseProperty() {
		if name == "END" {
			if len(prop.group) != 0 || len(prop.params) != 0 ||
				len(prop.values) != 1 || strings.ToUpper(prop.values[0]) != "VCARD" {
				return &Card{}, errors.New("malformed end tag")
			}
			return card, nil
		}
		card.m[name] = append(card.m[name], prop)
	}

	if err == io.EOF {
		return &Card{}, errors.New("unexpected end of input before ending card")
	}
	return &Card{}, err
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
		prop.group = nm
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
		prop.params = params
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
	prop.values = values

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
	key = strings.ToUpper(key)

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
