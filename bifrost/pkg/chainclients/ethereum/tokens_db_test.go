package ethereum

import (
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	. "gopkg.in/check.v1"
)

type EthereumTokenMetaTestSuite struct{}

var _ = Suite(
	&EthereumTokenMetaTestSuite{},
)

func (s *EthereumTokenMetaTestSuite) TestNewTokenMeta(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	dbTokenMeta, err := NewLevelDBTokenMeta(db)
	c.Assert(err, IsNil)
	c.Assert(dbTokenMeta, NotNil)
}

func (s *EthereumTokenMetaTestSuite) TestTokenMeta(c *C) {
	memStorage := storage.NewMemStorage()
	db, err := leveldb.Open(memStorage, nil)
	c.Assert(err, IsNil)
	TokenMeta, err := NewLevelDBTokenMeta(db)
	c.Assert(err, IsNil)
	c.Assert(TokenMeta, NotNil)

	c.Assert(TokenMeta.SaveTokenMeta("TKN", "0xa7d9ddbe1f17865597fbd27ec712455208b6b76d", 18), IsNil)

	key := TokenMeta.getTokenMetaKey("0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")
	c.Assert(key, Equals, fmt.Sprintf(prefixTokenMeta+"%s", "0xa7d9ddbe1f17865597fbd27ec712455208b6b76d"))

	tm, err := TokenMeta.GetTokenMeta("0xa7d9ddbe1f17865597fbd27ec712455208b6b76d")
	c.Assert(err, IsNil)
	c.Assert(tm, NotNil)

	ntm, err := TokenMeta.GetTokenMeta("0xa7d9ddbs1f17865597fbd27ec712455208b6b76d")
	c.Assert(err, IsNil)
	c.Assert(ntm.IsEmpty(), Equals, true)

	c.Assert(TokenMeta.SaveTokenMeta("TRN", "0xa7d9ddbs1f17865597fbd27ec712455208b6b76d", 18), IsNil)

	TokenMetas, err := TokenMeta.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(TokenMetas, HasLen, 2)
}
