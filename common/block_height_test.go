package common

import (
	. "gopkg.in/check.v1"

	"github.com/cosmos/cosmos-sdk/store"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"

	"gitlab.com/thorchain/thornode/common/cosmos"
)

type BlockHeightSuite struct{}

var _ = Suite(&BlockHeightSuite{})

func (BlockHeightSuite) TestBlockHeight(c *C) {
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ctx := cosmos.NewContext(ms, tmproto.Header{ChainID: "thorchain"}, false, log.NewNopLogger())
	ctx = ctx.WithBlockHeight(18)

	c.Assert(BlockHeight(ctx), Equals, int64(18))
}
