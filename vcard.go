// Copyright 2018 Ian Johnson
//
// This file is part of vcard. Vcard is free software: you are free to use it
// for any purpose, make modified versions and share it with others, subject
// to the terms of the Apache license (version 2.0), a copy of which is
// provided alongside this project.

package vcard

import (
	"bufio"
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
// property name. If the name does not correspond to a property, the result is
// nil.
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
// fits within 77 bytes. As with UnfoldedString, the order of properties in
// the result is undefined, except for VERSION, which will always come first if
// it is present.
func (c *Card) String() string {
	return Fold(c.UnfoldedString(), 77)
}

// UnfoldedString returns the card in vCard syntax, but without folding any
// lines or using the "\r\n" line ending (it just uses the normal '\n'). The
// order of the properties in the returned string is undefined, except that
// the VERSION property (if present) will always be first.
func (c *Card) UnfoldedString() string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, "BEGIN:VCARD")
	// If the VERSION property is present, we need to print that first.
	version, ok := c.m["VERSION"]
	// This implementation doesn't behave well if the version property
	// appears more than once, and it ignores any group or parameters, but
	// since no standard vCard will do any of that it seems fine to ignore
	// these cases.
	if ok && len(version) > 0 {
		sb.WriteString("VERSION:")
		writeValues(sb, version[0].values)
		sb.WriteRune('\n')
	}
	for name, props := range c.m {
		// We already wrote the VERSION property above.
		if name == "VERSION" {
			continue
		}

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
			fmt.Fprint(w, ",")
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

// ParseError is the error type returned when an error occurs during parsing.
type ParseError struct {
	Line int // the line on which the error occurred
	msg  string
}

func (p ParseError) Error() string {
	return fmt.Sprintf("on line %v: %v", p.Line, p.msg)
}

// Message returns the error message returned by Error without any line
// information.
func (p ParseError) Message() string {
	return p.msg
}

// ParseAll parses as many vCards from the given input as possible, until EOF
// is reached or a parsing error occurs. If parsing fails at any point, the
// returned slice will contain any cards that were successfully parsed
// before the error.
//
// This function is equivalent to wrapping the reader in a bufio.Reader (for
// efficiency), creating a Parser and repeatedly calling the Next method until
// it fails. Thus, it is sensitive to minor details like empty lines in a file
// (which will cause a parsing error); for more control over such details, use
// a Parser directly.
func ParseAll(r io.Reader) ([]*Card, error) {
	var cards []*Card
	p := NewParser(bufio.NewReader(r))

	for card, err := p.Next(); err != io.EOF; card, err = p.Next() {
		if err != nil {
			return cards, err
		}
		cards = append(cards, card)
	}

	return cards, nil
}

// Parser is a parser for vCard data that reads a series of cards from an
// underlying reader.
type Parser struct {
	r *UnfoldingReader
}

// NewParser returns a new parser that takes data from a reader. The parser
// takes care of unfolding the input data, so there is no need to wrap a
// reader with an UnfoldingReader before passing it to this function.
func NewParser(r io.Reader) *Parser {
	return &Parser{r: NewUnfoldingReader(r)}
}

// Next parses and returns the next available card.
func (p *Parser) Next() (*Card, error) {
	card := &Card{m: make(map[string][]Property)}

	line := p.r.Line()
	name, prop, err := p.parseProperty()
	if err != nil {
		return &Card{}, err
	} else if name != "BEGIN" || len(prop.group) != 0 || len(prop.params) != 0 ||
		len(prop.values) != 1 || strings.ToUpper(prop.values[0]) != "VCARD" {
		return &Card{}, ParseError{line, "expected beginning of card"}
	}

	line = p.r.Line()
	name, prop, err = p.parseProperty()
	for err == nil {
		if name == "END" {
			if len(prop.group) != 0 || len(prop.params) != 0 ||
				len(prop.values) != 1 || strings.ToUpper(prop.values[0]) != "VCARD" {
				return &Card{}, ParseError{line, "malformed end tag"}
			}
			return card, nil
		}
		card.m[name] = append(card.m[name], prop)

		line = p.r.Line()
		name, prop, err = p.parseProperty()
	}

	if err == io.EOF {
		return &Card{}, ParseError{p.r.Line(), "unexpected end of input before ending card"}
	}
	return &Card{}, err
}

// parseProperty parses a single property.
func (p *Parser) parseProperty() (name string, prop Property, err error) {
	// Parse name (or group).
	nm, err := p.parseName("expected property name")
	if err != nil {
		return "", Property{}, err
	}

	line := p.r.Line()
	b, err := p.demandByte("expected ';' or ':'")
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
		line = p.r.Line()
		b, err = p.demandByte("expected ';' or ':'")
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
		line = p.r.Line()
		b, err = p.demandByte("expected ':'")
	}

	if err != nil {
		return "", Property{}, err
	}
	if b != ':' {
		return "", Property{}, ParseError{line, "expected ':'"}
	}

	values, err := p.parsePropertyValues()
	if err != nil {
		return "", Property{}, err
	}
	prop.values = values

	line = p.r.Line()
	b, err = p.r.ReadByte()
	if err == io.EOF {
		return name, prop, nil
	} else if err != nil {
		return "", Property{}, err
	}
	if b != '\n' {
		return "", Property{}, ParseError{line, fmt.Sprintf("unexpected character %q after property value", b)}
	}
	return name, prop, nil
}

// parsePropertyValues parses several property values, separated by commas.
func (p *Parser) parsePropertyValues() ([]string, error) {
	var values []string

	value, err := p.parsePropertyValue()
	for err == nil {
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
		value, err = p.parsePropertyValue()
	}
	return nil, err
}

// parsePropertyValue parses a single property value. Since a property value
// may be empty, the returned error may be nil even if the returned string
// is empty.
func (p *Parser) parsePropertyValue() (string, error) {
	var bs []byte

	b, err := p.r.PeekByte()
	for err == nil {
		if !isValueChar(b) {
			return string(bs), nil
		}
		p.r.ReadByte()
		if b == '\\' {
			line := p.r.Line()
			b2, err := p.demandByte("expected escaped character")
			if err != nil {
				return "", err
			}
			if b2 == ',' || b2 == '\\' || b2 == ':' {
				bs = append(bs, b2)
			} else if b2 == 'n' {
				bs = append(bs, '\n')
			} else if b2 == ';' {
				bs = append(bs, '\\', ';')
			} else {
				return "", ParseError{line, fmt.Sprintf("%q cannot be escaped", b2)}
			}
		} else {
			bs = append(bs, b)
		}
		b, err = p.r.PeekByte()
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
func (p *Parser) parseParameters() (map[string][]string, error) {
	params := make(map[string][]string)

	key, values, err := p.parseParameter()
	for err == nil {
		params[key] = append(params[key], values...)

		b, err := p.r.PeekByte()
		if err == io.EOF {
			return params, nil
		} else if err != nil {
			return nil, err
		} else if b != ';' {
			return params, nil
		}
		p.r.ReadByte()
		key, values, err = p.parseParameter()
	}
	return nil, err
}

// parseParameter parses a single property parameter. If the returned error
// is nil, then the key and values will both be non-nil.
func (p *Parser) parseParameter() (key string, values []string, err error) {
	key, err = p.parseName("expected parameter name")
	if err != nil {
		return "", nil, err
	}
	key = strings.ToUpper(key)

	msg := fmt.Sprintf("expected '=' after parameter name %v", key)
	line := p.r.Line()
	b, err := p.demandByte(msg)
	if err != nil {
		return "", nil, err
	} else if b != '=' {
		return "", nil, ParseError{line, msg}
	}

	value, err := p.parseParameterValue()
	for err == nil {
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
		value, err = p.parseParameterValue()
	}
	return "", nil, err
}

// parseParameterValue parses a single property parameter value. The returned
// string may be empty even if the error is non-nil, since parameter values
// may be empty.
func (p *Parser) parseParameterValue() (string, error) {
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
func (p *Parser) parseQuotedParameterValue() (string, error) {
	var bs []byte

	line := p.r.Line()
	b, err := p.r.ReadByte()
	for err == nil {
		if b == '"' {
			return string(bs), nil
		} else if !isQuoteSafeChar(b) {
			return "", ParseError{line, fmt.Sprintf("unexpected byte %q in quoted parameter value", b)}
		}
		bs = append(bs, b)
		line = p.r.Line()
		b, err = p.r.ReadByte()
	}

	if err != nil && err != io.EOF {
		return "", err
	}
	return "", ParseError{line, "unexpected end of quoted parameter value"}
}

// isQuoteSafeChar returns whether the given byte may appear within a quoted
// parameter value.
func isQuoteSafeChar(b byte) bool {
	return b == ' ' || b == '\t' || b == '!' || '"' < b
}

// parseUnquotedParameterValue parses a parameter value not enclosed in double
// quotes.
func (p *Parser) parseUnquotedParameterValue() (string, error) {
	var bs []byte

	b, err := p.r.PeekByte()
	for err == nil {
		if !isSafeChar(b) {
			return string(bs), nil
		}
		p.r.ReadByte()
		bs = append(bs, b)
		b, err = p.r.PeekByte()
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
	// character; not including it makes the parsing logic a bit easier.
	return b == ' ' || b == '\t' || b == '!' || ('"' < b && b != ';' && b != ':' && b != ',')
}

// parseName parses anything that has the format of a property name, group
// or parameter name. If the parsed name is empty but no other error occurred,
// an error will be returned wrapping the given string.
func (p *Parser) parseName(missing string) (string, error) {
	var bs []byte

	line := p.r.Line()
	b, err := p.r.PeekByte()
	for err == nil {
		if ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') || b == '-' {
			bs = append(bs, b)
		} else if 'a' <= b && b <= 'z' {
			// Convert b to uppercase.
			bs = append(bs, b+'A'-'a')
		} else {
			break
		}
		p.r.ReadByte()
		line = p.r.Line()
		b, err = p.r.PeekByte()
	}

	if err != nil {
		return string(bs), err
	} else if len(bs) == 0 {
		return "", ParseError{line, missing}
	}
	return string(bs), nil
}

// demandByte reads the next byte according to readByte, but converts an EOF
// error into a ParseError wrapping the given string.
func (p *Parser) demandByte(missing string) (b byte, err error) {
	line := p.r.Line()
	b, err = p.r.ReadByte()
	if err == io.EOF {
		return 0, ParseError{line, err.Error()}
	}
	return
}
