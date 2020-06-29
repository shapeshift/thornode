package keeperv1

import "gitlab.com/thorchain/thornode/common/cosmos"

// SetActiveObserver set the given addr as an active observer address
func (k KVStore) SetActiveObserver(ctx cosmos.Context, addr cosmos.AccAddress) {
	k.set(ctx, k.GetKey(ctx, prefixActiveObserver, addr.String()), addr)
}

// RemoveActiveObserver remove the given address from active observer
func (k KVStore) RemoveActiveObserver(ctx cosmos.Context, addr cosmos.AccAddress) {
	k.del(ctx, k.GetKey(ctx, prefixActiveObserver, addr.String()))
}

// IsActiveObserver check the given account address, whether they are active
func (k KVStore) IsActiveObserver(ctx cosmos.Context, addr cosmos.AccAddress) bool {
	return k.has(ctx, k.GetKey(ctx, prefixActiveObserver, addr.String()))
}

// GetObservingAddresses - get list of observed addresses. This is a list of
// addresses that have recently contributed via observing a tx that got 2/3rds
// majority
func (k KVStore) GetObservingAddresses(ctx cosmos.Context) ([]cosmos.AccAddress, error) {
	record := make([]cosmos.AccAddress, 0)
	_, err := k.get(ctx, k.GetKey(ctx, prefixObservingAddresses, ""), &record)
	return record, err
}

// AddObservingAddresses - add a list of addresses that have been helpful in
// getting enough observations to process an inbound tx.
func (k KVStore) AddObservingAddresses(ctx cosmos.Context, inAddresses []cosmos.AccAddress) error {
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

	k.set(ctx, k.GetKey(ctx, prefixObservingAddresses, ""), uniq)
	return nil
}

// ClearObservingAddresses - clear all observing addresses
func (k KVStore) ClearObservingAddresses(ctx cosmos.Context) {
	k.del(ctx, k.GetKey(ctx, prefixObservingAddresses, ""))
}
