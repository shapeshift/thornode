//go:build regtest
// +build regtest

package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

func migrateStoreV86(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV88(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV102(ctx cosmos.Context, mgr Manager) {}

func migrateStoreV103(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV106(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV108(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV109(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV110(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV111(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV113(ctx cosmos.Context, mgr *Mgrs) {}

func migrateStoreV114(ctx cosmos.Context, mgr *Mgrs) {}

// migrateStoreV116 subset of mainnet migration
func migrateStoreV116(ctx cosmos.Context, mgr *Mgrs) {
	bondRuneOver := cosmos.NewUint(6936522592883)
	asgardRuneUnder := cosmos.NewUint(5082320319988)
	thorchainRuneOver := cosmos.NewUint(100000000)

	actions := []ModuleBalanceAction{
		// send rune from bond oversolvency to fix asgard insolvency
		{
			ModuleName:     BondName,
			RuneRecipient:  AsgardName,
			RuneToTransfer: asgardRuneUnder,
			SynthsToBurn:   common.Coins{},
		},

		// send remaining bond rune oversolvency to reserve
		{
			ModuleName:     BondName,
			RuneRecipient:  ReserveName,
			RuneToTransfer: common.SafeSub(bondRuneOver, asgardRuneUnder),
			SynthsToBurn:   common.Coins{},
		},

		// transfer rune from thorchain to reserve to clear thorchain balances
		{
			ModuleName:     ModuleName,
			RuneRecipient:  ReserveName,
			RuneToTransfer: thorchainRuneOver,
			SynthsToBurn:   common.Coins{},
		},

		// burn synths from asgard to fix oversolvencies
		{
			ModuleName:     AsgardName,
			RuneRecipient:  AsgardName, // noop
			RuneToTransfer: cosmos.ZeroUint(),
			SynthsToBurn: common.Coins{
				{
					Asset:  common.AVAXAsset.GetSyntheticAsset(),
					Amount: cosmos.NewUint(1000001),
				},
			},
		},
	}

	processModuleBalanceActions(ctx, mgr.Keeper(), actions)
}

func migrateStoreV117(ctx cosmos.Context, mgr *Mgrs) {}
