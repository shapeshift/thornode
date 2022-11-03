package cmd

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/codec"
	bech32 "github.com/cosmos/cosmos-sdk/types/bech32/legacybech32" // nolint SA1019 deprecated
	"github.com/decred/dcrd/dcrec/edwards"
	"github.com/tendermint/tendermint/crypto/ed25519"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/x/thorchain"
)

func TestPackage(t *testing.T) { TestingT(t) }

type ED25519TestSuite struct{}

var _ = Suite(&ED25519TestSuite{})

func (s *ED25519TestSuite) SetUpTest(c *C) {
	thorchain.SetupConfigForTest()
}

func (*ED25519TestSuite) TestGetEd25519Keys(c *C) {
	thorchain.SetupConfigForTest()
	mnemonic := `grape safe sound obtain bachelor festival profit iron meat moon exit garbage chapter promote noble grocery blood letter junk click mesh arm shop decorate`
	result, err := mnemonicToEddKey(mnemonic, "")
	c.Assert(err, IsNil)
	// now we test the ed25519 key can sign and verify
	_, pk, err := edwards.PrivKeyFromScalar(edwards.Edwards(), result)
	c.Assert(err, IsNil)
	pkey := ed25519.PubKey(pk.Serialize())
	tmp, err := codec.FromTmPubKeyInterface(pkey)
	c.Assert(err, IsNil)
	// nolint
	pubKey, err := bech32.MarshalPubKey(bech32.AccPK, tmp)
	c.Assert(err, IsNil)
	c.Assert(pubKey, Equals, "tthorpub1zcjduepqkh2q3agpupf9w3kqpgqfe0n3crtn8jljzds777x4x9tw9tngmk6s4vfcz5")

	mnemonic = `invalid grape safe sound obtain bachelor festival profit iron meat moon exit garbage chapter promote noble grocery blood letter junk click mesh arm shop decorate`
	result, err = mnemonicToEddKey(mnemonic, "")
	c.Assert(err, NotNil)
	c.Assert(result, IsNil)
}
