package thorchain

import (
	"fmt"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/x/thorchain/keeper"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

func migrateStoreV2(ctx cosmos.Context, keeper keeper.Keeper) error {
	pools, err := keeper.GetPools(ctx)
	if err != nil {
		return fmt.Errorf("fail to get pool:%w", err)
	}
	for _, p := range pools {
		if p.Asset.Symbol.IsMiniToken() {
			// remove pool
			keeper.RemovePool(ctx, p.Asset)
		}
	}

	// remove the coin from all our vault (asgard and yggdrasil)
	iter := keeper.GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
		}

		if vault.Coins.IsEmpty() {
			continue
		}

		coinsToRemove := common.Coins{}
		for _, c := range vault.Coins {
			if c.Asset.Symbol.IsMiniToken() {
				coinsToRemove = append(coinsToRemove, c)
			}
		}
		if coinsToRemove.IsEmpty() {
			continue
		}

		vault.SubFunds(coinsToRemove)
		if err := keeper.SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}

	return nil
}

func migrateStoreV4(ctx cosmos.Context, keeper keeper.Keeper) error {
	nodeIter := keeper.GetNodeAccountIterator(ctx)
	defer nodeIter.Close()
	var activeNodeAddresses []cosmos.AccAddress
	for ; nodeIter.Valid(); nodeIter.Next() {
		var nodeAccount NodeAccount
		if err := keeper.Cdc().UnmarshalBinaryBare(nodeIter.Value(), &nodeAccount); err != nil {
			ctx.Logger().Error("fail to unmarshal node account", "error", err)
			continue
		}
		// ignore those account that had left
		if nodeAccount.Status == NodeDisabled || nodeAccount.RequestedToLeave {
			continue
		}
		activeNodeAddresses = append(activeNodeAddresses, nodeAccount.NodeAddress)
	}

	iter := keeper.GetVaultIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var vault Vault
		if err := keeper.Cdc().UnmarshalBinaryBare(iter.Value(), &vault); err != nil {
			ctx.Logger().Error("fail to unmarshal vault", "error", err)
			continue
		}

		if vault.Coins.IsEmpty() {
			continue
		}

		if vault.Type != AsgardVault {
			continue
		}
		pubKeys, err := vault.GetMembers(activeNodeAddresses)
		if err != nil {
			ctx.Logger().Error("fail to get members", "error", err)
		}
		if !types.HasSuperMajority(len(pubKeys), len(vault.Membership)) {
			ctx.Logger().Info("don't have 2/3 majority of signers, can't update vault")
			continue
		}
		if vault.Status == InactiveVault {
			vault.Status = RetiringVault
		}
		if err := keeper.SetVault(ctx, vault); err != nil {
			ctx.Logger().Error("fail to save vault", "error", err)
		}
	}
	return nil
}
