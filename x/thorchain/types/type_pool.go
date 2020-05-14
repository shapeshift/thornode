package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"gitlab.com/thorchain/thornode/common"
)

// PoolStatus is an indication of what the pool state is
type PoolStatus int

// |    State    | Swap | Stake | Unstake  | Refunding |
// | ----------- | ---- | ----- | -------- | --------- |
// | `bootstrap` | no   | yes   | yes      | Refund Invalid Stakes && all Swaps |
// | `enabled`   | yes  | yes   | yes      | Refund Invalid Tx |
// | `suspended` | no   | no    | no       | Refund all |
const (
	Enabled PoolStatus = iota
	Bootstrap
	Suspended
)

var poolStatusStr = map[string]PoolStatus{
	"Enabled":   Enabled,
	"Bootstrap": Bootstrap,
	"Suspended": Suspended,
}

// String implement stringer
func (ps PoolStatus) String() string {
	for key, item := range poolStatusStr {
		if item == ps {
			return key
		}
	}
	return ""
}

func (ps PoolStatus) Valid() error {
	if ps.String() == "" {
		return fmt.Errorf("Invalid pool status")
	}
	return nil
}

// MarshalJSON marshal PoolStatus to JSON in string form
func (ps PoolStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(ps.String())
}

// UnmarshalJSON convert string form back to PoolStatus
func (ps *PoolStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*ps = GetPoolStatus(s)
	return nil
}

// GetPoolStatus from string
func GetPoolStatus(ps string) PoolStatus {
	for key, item := range poolStatusStr {
		if strings.EqualFold(key, ps) {
			return item
		}
	}

	return Suspended
}

// Pool is a struct that contains all the metadata of a pooldata
// This is the structure THORNode will saved to the key value store
type Pool struct {
	BalanceRune  sdk.Uint       `json:"balance_rune"`  // how many RUNE in the pool
	BalanceAsset sdk.Uint       `json:"balance_asset"` // how many asset in the pool
	Asset        common.Asset   `json:"asset"`         // what's the asset's asset
	PoolUnits    sdk.Uint       `json:"pool_units"`    // total units of the pool
	PoolAddress  common.Address `json:"pool_address"`  // bnb liquidity pool address
	Status       PoolStatus     `json:"status"`        // status
}

type Pools []Pool

// NewPool Returns a new Pool
func NewPool() Pool {
	return Pool{
		BalanceRune:  sdk.ZeroUint(),
		BalanceAsset: sdk.ZeroUint(),
		PoolUnits:    sdk.ZeroUint(),
		Status:       Enabled,
	}
}

func (ps Pool) Valid() error {
	if ps.Empty() {
		return errors.New("pool asset cannot be empty")
	}
	return nil
}

func (ps Pool) IsEnabled() bool {
	return ps.Status == Enabled
}

func (ps Pool) Empty() bool {
	return ps.Asset.IsEmpty()
}

// String implement fmt.Stringer
func (ps Pool) String() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintln("rune-balance: " + ps.BalanceRune.String()))
	sb.WriteString(fmt.Sprintln("asset-balance: " + ps.BalanceAsset.String()))
	sb.WriteString(fmt.Sprintln("asset: " + ps.Asset.String()))
	sb.WriteString(fmt.Sprintln("pool-units: " + ps.PoolUnits.String()))
	sb.WriteString(fmt.Sprintln("status: " + ps.Status.String()))
	return sb.String()
}

// EnsureValidPoolStatus
func (ps Pool) EnsureValidPoolStatus(msg sdk.Msg) error {
	switch ps.Status {
	case Enabled:
		return nil
	case Bootstrap:
		switch msg.(type) {
		case MsgSwap:
			return errors.New("pool is in bootstrap status, can't swap")
		default:
			return nil
		}
	case Suspended:
		return errors.New("pool suspended")
	default:
		return fmt.Errorf("unknown pool status,%s", ps.Status)
	}
}

// convert a specific amount of asset amt into its rune value
func (ps Pool) AssetValueInRune(amt sdk.Uint) sdk.Uint {
	if ps.BalanceRune.IsZero() || ps.BalanceAsset.IsZero() {
		return sdk.ZeroUint()
	}
	return common.GetShare(ps.BalanceRune, ps.BalanceAsset, amt)
}

// convert a specific amount of rune amt into its asset value
func (ps Pool) RuneValueInAsset(amt sdk.Uint) sdk.Uint {
	if ps.BalanceRune.IsZero() || ps.BalanceAsset.IsZero() {
		return sdk.ZeroUint()
	}
	return common.GetShare(ps.BalanceAsset, ps.BalanceRune, amt)
}
