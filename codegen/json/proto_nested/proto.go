package proto_nested

import jsonrt "github.com/rodrigocelsius/go-nitro/json/runtime/v2"

// Compares two parse strategies for nested struct fields:
//
// Strategy A (current): delimit the nested object's byte range via
// SkipValue, then create a NEW Parser over that slice and parse it
// fully — walks the nested object bytes twice.
//
// Strategy B (proposed): parse the nested object directly, continuing
// on the SAME Parser (same position), with no prior SkipValue —
// walks the bytes once.

type Address struct {
	Street string
	City   string
}

// ParseJSON_A is the current strategy: SkipValue + new Parser.
func (a *Address) ParseJSON_A(data []byte) bool {
	p := jsonrt.NewParser(data)
	return a.parseBodyA(p)
}

func (a *Address) parseBodyA(p *jsonrt.Parser) bool {
	if !p.ExpectByte('{') {
		return false
	}
	if p.PeekByte('}') {
		p.Pos++
		return true
	}
	for {
		key, ok := p.ParseKey()
		if !ok {
			return false
		}
		switch key {
		case "street":
			s, ok := p.ParseString()
			if !ok {
				return false
			}
			a.Street = s
		case "city":
			s, ok := p.ParseString()
			if !ok {
				return false
			}
			a.City = s
		default:
			if !p.SkipValue() {
				return false
			}
		}
		if p.PeekByte(',') {
			p.Pos++
			continue
		}
		break
	}
	return p.ExpectByte('}')
}

// ParseJSON_B is the proposed strategy: continue on the same Parser,
// with no prior SkipValue. The parent struct calls parseBodyA/B
// directly, passing its own *Parser along.
func (a *Address) ParseJSON_B(p *jsonrt.Parser) bool {
	return a.parseBodyA(p) // body parse logic is identical;
	// the difference is HOW the parent calls it (see Order below)
}

type Order struct {
	ID      int
	Address Address
}

// OrderParseJSON_A simulates CURRENT generated code: uses SkipValue to
// delimit the nested field range, then calls ParseJSON_A (which
// creates a new Parser and re-parses from scratch).
func (o *Order) OrderParseJSON_A(data []byte) bool {
	p := jsonrt.NewParser(data)
	if !p.ExpectByte('{') {
		return false
	}
	if p.PeekByte('}') {
		p.Pos++
		return true
	}
	for {
		key, ok := p.ParseKey()
		if !ok {
			return false
		}
		switch key {
		case "id":
			n, ok := p.ParseInt()
			if !ok {
				return false
			}
			o.ID = int(n)
		case "address":
			start := p.Pos
			if !p.SkipValue() {
				return false
			}
			if !(&o.Address).ParseJSON_A(p.Data[start:p.Pos]) {
				return false
			}
		default:
			if !p.SkipValue() {
				return false
			}
		}
		if p.PeekByte(',') {
			p.Pos++
			continue
		}
		break
	}
	return p.ExpectByte('}')
}

// OrderParseJSON_B simulates PROPOSED generated code: calls
// ParseJSON_B directly, passing the shared parser along —
// no SkipValue, no new Parser, no re-walking nested bytes.
func (o *Order) OrderParseJSON_B(data []byte) bool {
	p := jsonrt.NewParser(data)
	if !p.ExpectByte('{') {
		return false
	}
	if p.PeekByte('}') {
		p.Pos++
		return true
	}
	for {
		key, ok := p.ParseKey()
		if !ok {
			return false
		}
		switch key {
		case "id":
			n, ok := p.ParseInt()
			if !ok {
				return false
			}
			o.ID = int(n)
		case "address":
			if !(&o.Address).ParseJSON_B(p) {
				return false
			}
		default:
			if !p.SkipValue() {
				return false
			}
		}
		if p.PeekByte(',') {
			p.Pos++
			continue
		}
		break
	}
	return p.ExpectByte('}')
}
