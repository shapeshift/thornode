package common

import (
	. "gopkg.in/check.v1"
)

type SymbolSuite struct{}

var _ = Suite(&SymbolSuite{})

func (s SymbolSuite) TestSymbol(c *C) {
	sym, err := NewSymbol("rune-a1f")
	c.Assert(err, IsNil)
	c.Check(sym.Equals(RuneA1FSymbol), Equals, true)
	c.Check(sym.IsEmpty(), Equals, false)
	c.Check(sym.String(), Equals, "RUNE-A1F")
	c.Check(sym.Ticker().Equals(Ticker("RUNE")), Equals, true)
}
