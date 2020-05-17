package thorclient

import (
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client/keys"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keys"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/x/thorchain"
)

type KeysSuite struct{}

var _ = Suite(&KeysSuite{})

func (*KeysSuite) SetUpSuite(c *C) {
	thorchain.SetupConfigForTest()
}

const (
	signerNameForTest     = `jack`
	signerPasswordForTest = `password`
)

func (*KeysSuite) setupKeysForTest(c *C) string {
	thorcliDir := filepath.Join(os.TempDir(), ".thorcli")
	kb, err := keys.NewKeyBaseFromDir(thorcliDir)
	c.Assert(err, IsNil)
	_, _, err = kb.CreateMnemonic(signerNameForTest, cKeys.English, signerPasswordForTest, cKeys.Secp256k1)
	c.Assert(err, IsNil)
	kb.CloseDB()
	return thorcliDir
}

func (ks *KeysSuite) TestNewKeys(c *C) {
	folder := ks.setupKeysForTest(c)
	defer func() {
		err := os.RemoveAll(folder)
		c.Assert(err, IsNil)
	}()

	k, info, err := GetKeybase(folder, "")
	c.Assert(err, NotNil)
	c.Assert(k, NotNil)
	c.Assert(info, IsNil)
	k, info, err = GetKeybase(folder, signerNameForTest)
	c.Assert(err, IsNil)
	c.Assert(k, NotNil)
	c.Assert(info, NotNil)
	ki := NewKeysWithKeybase(k, info, signerPasswordForTest)
	kInfo := ki.GetSignerInfo()
	c.Assert(kInfo, NotNil)
	c.Assert(kInfo.GetName(), Equals, signerNameForTest)
	priKey, err := ki.GetPrivateKey()
	c.Assert(err, IsNil)
	c.Assert(priKey, NotNil)
	c.Assert(priKey.Bytes(), HasLen, 37)
	kb := ki.GetKeybase()
	c.Assert(kb, NotNil)
	kb.CloseDB()
}
