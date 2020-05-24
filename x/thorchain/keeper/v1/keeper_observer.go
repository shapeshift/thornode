package thorchain

import cosmos "gitlab.com/thorchain/thornode/common/cosmos"

// SetActiveObserver set the given addr as an active observer address
func (k KVStoreV1) SetActiveObserver(ctx cosmos.Context, addr cosmos.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixActiveObserver, addr.String())
	ctx.Logger().Debug("set_active_observer", "key", key)
	store.Set([]byte(key), addr.Bytes())
}

// RemoveActiveObserver remove the given address from active observer
func (k KVStoreV1) RemoveActiveObserver(ctx cosmos.Context, addr cosmos.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixActiveObserver, addr.String())
	store.Delete([]byte(key))
}

// IsActiveObserver check the given account address, whether they are active
func (k KVStoreV1) IsActiveObserver(ctx cosmos.Context, addr cosmos.AccAddress) bool {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixActiveObserver, addr.String())
	ctx.Logger().Debug("is_active_observer", "key", key)
	return store.Has([]byte(key))
}

// GetObservingAddresses - get list of observed addresses. This is a list of
// addresses that have recently contributed via observing a tx that got 2/3rds
// majority
func (k KVStoreV1) GetObservingAddresses(ctx cosmos.Context) ([]cosmos.AccAddress, error) {
	key := k.GetKey(ctx, prefixObservingAddresses, "")

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return make([]cosmos.AccAddress, 0), nil
	}

	bz := store.Get([]byte(key))
	var addresses []cosmos.AccAddress
	if err := k.cdc.UnmarshalBinaryBare(bz, &addresses); err != nil {
		return nil, dbError(ctx, "Unmarshal: observer", err)
	}
	return addresses, nil
}

// AddObservingAddresses - add a list of addresses that have been helpful in
// getting enough observations to process an inbound tx.
func (k KVStoreV1) AddObservingAddresses(ctx cosmos.Context, inAddresses []cosmos.AccAddress) error {
	if len(inAddresses) == 0 {
		return nil
	}

	// combine addresses
	curr, err := k.GetObservingAddresses(ctx)
	if err != nil {
		return err
	}
	all := append(curr, inAddresses...)

	// ensure uniqueness
	uniq := make([]cosmos.AccAddress, 0, len(all))
	m := make(map[string]bool)
	for _, val := range all {
		if _, ok := m[val.String()]; !ok {
			m[val.String()] = true
			uniq = append(uniq, val)
		}
	}

	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixObservingAddresses, "")
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(uniq))
	return nil
}

// ClearObservingAddresses - clear all observing addresses
func (k KVStoreV1) ClearObservingAddresses(ctx cosmos.Context) {
	key := k.GetKey(ctx, prefixObservingAddresses, "")
	store := ctx.KVStore(k.storeKey)
	store.Delete([]byte(key))
}
