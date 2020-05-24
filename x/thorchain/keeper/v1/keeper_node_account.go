package thorchain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
)

// TotalActiveNodeAccount count the number of active node account
func (k KVStoreV1) TotalActiveNodeAccount(ctx cosmos.Context) (int, error) {
	activeNodes, err := k.ListActiveNodeAccounts(ctx)
	return len(activeNodes), err
}

// ListNodeAccountsWithBond - gets a list of all node accounts that have bond
func (k KVStoreV1) ListNodeAccountsWithBond(ctx cosmos.Context) (NodeAccounts, error) {
	nodeAccounts := make(NodeAccounts, 0)
	naIterator := k.GetNodeAccountIterator(ctx)
	defer naIterator.Close()
	for ; naIterator.Valid(); naIterator.Next() {
		var na NodeAccount
		if err := k.cdc.UnmarshalBinaryBare(naIterator.Value(), &na); err != nil {
			return nodeAccounts, dbError(ctx, "Unmarshal: node account", err)
		}
		if !na.Bond.IsZero() {
			nodeAccounts = append(nodeAccounts, na)
		}
	}
	return nodeAccounts, nil
}

// ListNodeAccountsByStatus - get a list of node accounts with the given status
// if status = NodeUnknown, then it return everything
func (k KVStoreV1) ListNodeAccountsByStatus(ctx cosmos.Context, status NodeStatus) (NodeAccounts, error) {
	nodeAccounts := make(NodeAccounts, 0)
	naIterator := k.GetNodeAccountIterator(ctx)
	defer naIterator.Close()
	for ; naIterator.Valid(); naIterator.Next() {
		var na NodeAccount
		if err := k.cdc.UnmarshalBinaryBare(naIterator.Value(), &na); err != nil {
			return nodeAccounts, dbError(ctx, "Unmarshal: node account", err)
		}
		if na.Status == status {
			nodeAccounts = append(nodeAccounts, na)
		}
	}
	return nodeAccounts, nil
}

// ListActiveNodeAccounts - get a list of active node accounts
func (k KVStoreV1) ListActiveNodeAccounts(ctx cosmos.Context) (NodeAccounts, error) {
	return k.ListNodeAccountsByStatus(ctx, NodeActive)
}

// GetMinJoinVersion - get min version to join. Min version is the most popular version
func (k KVStoreV1) GetMinJoinVersion(ctx cosmos.Context) semver.Version {
	type tmpVersionInfo struct {
		version semver.Version
		count   int
	}
	vCount := make(map[string]tmpVersionInfo, 0)
	nodes, err := k.ListActiveNodeAccounts(ctx)
	if err != nil {
		_ = dbError(ctx, "Unable to list active node accounts", err)
		return semver.Version{}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].Version.LT(nodes[j].Version)
	})
	for _, na := range nodes {
		v, ok := vCount[na.Version.String()]
		if ok {
			v.count = v.count + 1
			vCount[na.Version.String()] = v

		} else {
			vCount[na.Version.String()] = tmpVersionInfo{
				version: na.Version,
				count:   1,
			}
		}
		// assume all versions are  backward compatible
		for k, v := range vCount {
			if v.version.LT(na.Version) {
				v.count = v.count + 1
				vCount[k] = v
			}
		}
	}
	totalCount := len(nodes)
	version := semver.Version{}

	for _, info := range vCount {
		// skip those version that doesn't have majority
		if !HasSuperMajority(info.count, totalCount) {
			continue
		}
		if info.version.GT(version) {
			version = info.version
		}

	}
	return version
}

// GetLowestActiveVersion - get version number of lowest active node
func (k KVStoreV1) GetLowestActiveVersion(ctx cosmos.Context) semver.Version {
	nodes, err := k.ListActiveNodeAccounts(ctx)
	if err != nil {
		_ = dbError(ctx, "Unable to list active node accounts", err)
		return constants.SWVersion
	}
	if len(nodes) > 0 {
		version := nodes[0].Version
		for _, na := range nodes {
			if na.Version.LT(version) {
				version = na.Version
			}
		}
		return version
	}
	return constants.SWVersion
}

// GetNodeAccount try to get node account with the given address from db
func (k KVStoreV1) GetNodeAccount(ctx cosmos.Context, addr cosmos.AccAddress) (NodeAccount, error) {
	ctx.Logger().Debug("GetNodeAccount", "node account", addr.String())
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixNodeAccount, addr.String())
	if !store.Has([]byte(key)) {
		emptyPubKeySet := common.PubKeySet{
			Secp256k1: common.EmptyPubKey,
			Ed25519:   common.EmptyPubKey,
		}
		return NewNodeAccount(addr, NodeUnknown, emptyPubKeySet, "", cosmos.ZeroUint(), "", ctx.BlockHeight()), nil
	}

	payload := store.Get([]byte(key))
	var na NodeAccount
	if err := k.cdc.UnmarshalBinaryBare(payload, &na); err != nil {
		return na, dbError(ctx, "Unmarshal: node account", err)
	}
	return na, nil
}

// GetNodeAccountByPubKey try to get node account with the given pubkey from db
func (k KVStoreV1) GetNodeAccountByPubKey(ctx cosmos.Context, pk common.PubKey) (NodeAccount, error) {
	addr, err := pk.GetThorAddress()
	if err != nil {
		return NodeAccount{}, err
	}
	return k.GetNodeAccount(ctx, addr)
}

// GetNodeAccountByBondAddress go through data store to get node account by it's signer bnb address
func (k KVStoreV1) GetNodeAccountByBondAddress(ctx cosmos.Context, addr common.Address) (NodeAccount, error) {
	naIterator := k.GetNodeAccountIterator(ctx)
	defer naIterator.Close()
	for ; naIterator.Valid(); naIterator.Next() {
		var na NodeAccount
		if err := k.cdc.UnmarshalBinaryBare(naIterator.Value(), &na); err != nil {
			return na, dbError(ctx, "Unmarshal: node account", err)
		}
		if na.BondAddress.Equals(addr) {
			return na, nil
		}
	}

	return NodeAccount{}, nil
}

// SetNodeAccount save the given node account into datastore
func (k KVStoreV1) SetNodeAccount(ctx cosmos.Context, na NodeAccount) error {
	ctx.Logger().Debug("SetNodeAccount", "node account", na.String())
	if na.IsEmpty() {
		return nil
	}
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixNodeAccount, na.NodeAddress.String())
	if na.Status == NodeActive {
		if na.ActiveBlockHeight == 0 {
			// the na is active, and does not have a block height when they
			// became active. This must be the first block they are active, so
			// THORNode will set it now.
			na.ActiveBlockHeight = ctx.BlockHeight()
			k.ResetNodeAccountSlashPoints(ctx, na.NodeAddress) // reset slash points
		}
	}

	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(na))

	// When a node is in active status, THORNode need to add the observer address to active
	// if it is not , then THORNode could remove them
	if na.Status == NodeActive {
		k.SetActiveObserver(ctx, na.NodeAddress)
	} else {
		k.RemoveActiveObserver(ctx, na.NodeAddress)
	}
	return nil
}

// EnsureNodeKeysUnique check the given consensus pubkey and pubkey set against all the the node account
// return an error when it is overlap with any existing account
func (k KVStoreV1) EnsureNodeKeysUnique(ctx cosmos.Context, consensusPubKey string, pubKeys common.PubKeySet) error {
	iter := k.GetNodeAccountIterator(ctx)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var na NodeAccount
		if err := k.cdc.UnmarshalBinaryBare(iter.Value(), &na); err != nil {
			return dbError(ctx, "Unmarshal: node account", err)
		}
		if strings.EqualFold("", consensusPubKey) {
			return dbError(ctx, "", errors.New("Validator Consensus Key cannot be empty"))
		}
		if na.ValidatorConsPubKey == consensusPubKey {
			return dbError(ctx, "", fmt.Errorf("%s already exist", na.ValidatorConsPubKey))
		}
		if pubKeys.Equals(common.EmptyPubKeySet) {
			return dbError(ctx, "", errors.New("PubKeySet cannot be empty"))
		}
		if na.PubKeySet.Equals(pubKeys) {
			return dbError(ctx, "", fmt.Errorf("%s already exist", pubKeys))
		}
	}

	return nil
}

// GetNodeAccountIterator iterate node account
func (k KVStoreV1) GetNodeAccountIterator(ctx cosmos.Context) cosmos.Iterator {
	store := ctx.KVStore(k.storeKey)
	return cosmos.KVStorePrefixIterator(store, []byte(prefixNodeAccount))
}

// GetNodeAccountSlashPoints - get the slash points associated with the given
// node address
func (k KVStoreV1) GetNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress) (int64, error) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixNodeSlashPoints, addr.String())
	if !store.Has([]byte(key)) {
		return 0, nil
	}
	payload := store.Get([]byte(key))
	var pts int64
	if err := k.cdc.UnmarshalBinaryBare(payload, &pts); err != nil {
		return pts, dbError(ctx, "Unmarshal: node account slash points", err)
	}
	return pts, nil
}

// SetNodeAccountSlashPoints - set the slash points associated with the given
// node address and uint
func (k KVStoreV1) SetNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) {
	// make sure slash point doesn't go to negative
	if pts < 0 {
		return
	}
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixNodeSlashPoints, addr.String())
	store.Set([]byte(key), k.cdc.MustMarshalBinaryBare(pts))
}

// ResetNodeAccountSlashPoints - reset the slash points to zero for associated
// with the given node address
func (k KVStoreV1) ResetNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixNodeSlashPoints, addr.String())
	store.Delete([]byte(key))
}

// IncNodeAccountSlashPoints - increments the slash points associated with the
// given node address and uint
func (k KVStoreV1) IncNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	current, err := k.GetNodeAccountSlashPoints(ctx, addr)
	if err != nil {
		return err
	}
	k.SetNodeAccountSlashPoints(ctx, addr, current+pts)
	return nil
}

// DecNodeAccountSlashPoints - decrements the slash points associated with the
// given node address and uint
func (k KVStoreV1) DecNodeAccountSlashPoints(ctx cosmos.Context, addr cosmos.AccAddress, pts int64) error {
	current, err := k.GetNodeAccountSlashPoints(ctx, addr)
	if err != nil {
		return err
	}
	k.SetNodeAccountSlashPoints(ctx, addr, current-pts)
	return nil
}
