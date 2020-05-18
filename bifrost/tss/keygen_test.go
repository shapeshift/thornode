package tss

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client/keys"
	cKeys "github.com/cosmos/cosmos-sdk/crypto/keys"
	. "gopkg.in/check.v1"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/x/thorchain"
)

func TestTSSKeyGen(t *testing.T) { TestingT(t) }

type KeyGenTestSuite struct{}

var _ = Suite(&KeyGenTestSuite{})

func (*KeyGenTestSuite) SetUpSuite(c *C) {
	thorchain.SetupConfigForTest()
}

const (
	signerNameForTest     = `jack`
	signerPasswordForTest = `password`
)

func (*KeyGenTestSuite) setupKeysForTest(c *C) string {
	ns := strconv.Itoa(time.Now().Nanosecond())
	thorcliDir := filepath.Join(os.TempDir(), ns, ".thorcli")
	c.Logf("thorcliDir:%s", thorcliDir)
	kb, err := keys.NewKeyBaseFromDir(thorcliDir)
	c.Assert(err, IsNil)
	_, _, err = kb.CreateMnemonic(signerNameForTest, cKeys.English, signerPasswordForTest, cKeys.Secp256k1)
	c.Assert(err, IsNil)
	kb.CloseDB()
	return thorcliDir
}

func (kts *KeyGenTestSuite) TestNewTssKenGen(c *C) {
	folder := kts.setupKeysForTest(c)
	defer func() {
		err := os.RemoveAll(folder)
		c.Assert(err, IsNil)
	}()
	kb, info, err := thorclient.GetKeybase(folder, signerNameForTest)
	c.Assert(err, IsNil)
	k := thorclient.NewKeysWithKeybase(kb, info, signerPasswordForTest)
	c.Assert(k, NotNil)
	kg, err := NewTssKeyGen(k, nil)
	c.Assert(err, IsNil)
	c.Assert(kg, NotNil)
}
