package common

import (
	. "gopkg.in/check.v1"
)

type SymbolSuite struct{}

var _ = Suite(&SymbolSuite{})

func (s SymbolSuite) TestSymbol(c *C) {
	sym, err := NewSymbol("RUNE-67C")
	c.Assert(err, IsNil)
	c.Check(sym.Equals(Rune67CSymbol), Equals, true)
	c.Check(sym.IsEmpty(), Equals, false)
	c.Check(sym.String(), Equals, "RUNE-67C")
	c.Check(sym.Ticker().Equals(Ticker("RUNE")), Equals, true)
}
