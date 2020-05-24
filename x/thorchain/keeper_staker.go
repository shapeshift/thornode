package thorchain

import (
	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

type KeeperStaker interface {
	GetStakerIterator(ctx cosmos.Context, _ common.Asset) cosmos.Iterator
	GetStaker(ctx cosmos.Context, asset common.Asset, addr common.Address) (Staker, error)
	SetStaker(ctx cosmos.Context, staker Staker)
	RemoveStaker(ctx cosmos.Context, staker Staker)
}

// GetStakerIterator iterate stakers
func (k KVStoreV1) GetStakerIterator(ctx cosmos.Context, asset common.Asset) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixStaker, Staker{Asset: asset}.Key())
	return cosmos.KVStorePrefixIterator(store, []byte(key))
}

// GetStaker retrieve staker from the data store
func (k KVStoreV1) GetStaker(ctx cosmos.Context, asset common.Asset, addr common.Address) (Staker, error) {
	store := ctx.KVStore(k.storeKey)
	staker := Staker{
		Asset:       asset,
		RuneAddress: addr,
		Units:       cosmos.ZeroUint(),
		PendingRune: cosmos.ZeroUint(),
	}
	key := k.GetKey(ctx, prefixStaker, staker.Key())
	if !store.Has([]byte(key)) {
		return staker, nil
	}
	buf := store.Get([]byte(key))
	if err := k.cdc.UnmarshalBinaryBare(buf, &staker); err != nil {
		return staker, err
	}
	return staker, nil
}

// SetStaker store the staker to kvstore
func (k KVStoreV1) SetStaker(ctx cosmos.Context, staker Staker) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixStaker, staker.Key())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(staker))
}

// RemoveStaker remove the staker to kvstore
func (k KVStoreV1) RemoveStaker(ctx cosmos.Context, staker Staker) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixStaker, staker.Key())
	store.Delete([]byte(key))
}
