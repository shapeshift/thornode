package types

import (
	"fmt"
	"strings"

	"github.com/blang/semver"

	"gitlab.com/thorchain/thornode/common"
	cosmos "gitlab.com/thorchain/thornode/common/cosmos"
)

// Query Result Payload for a pools query
type QueryResPools []Pool

// implement fmt.Stringer
func (n QueryResPools) String() string {
	var assets []string
	for _, record := range n {
		assets = append(assets, record.Asset.String())
	}
	return strings.Join(assets, "\n")
}

type QueryResHeights struct {
	Chain            common.Chain `json:"chain"`
	LastChainHeight  int64        `json:"lastobservedin"`
	LastSignedHeight int64        `json:"lastsignedout"`
	Thorchain        int64        `json:"thorchain"`
}

func (h QueryResHeights) String() string {
	return fmt.Sprintf("Chain: %d, Signed: %d, THORChain: %d", h.LastChainHeight, h.LastSignedHeight, h.Thorchain)
}

type QueryOutQueue struct {
	Total int64 `json:"total"`
}

func (h QueryOutQueue) String() string {
	return fmt.Sprintf("Total: %d", h.Total)
}

type QueryNodeAccountPreflightCheck struct {
	Status      NodeStatus `json:"status"`
	Description string     `json:"reason"`
	Code        int        `json:"code"`
}

// implement fmt.Stringer
func (n QueryNodeAccountPreflightCheck) String() string {
	sb := strings.Builder{}
	sb.WriteString("Result Status:" + n.Status.String() + "\n")
	sb.WriteString("Description:" + n.Description + "\n")
	return sb.String()
}

// query keygen, displays signed keygen requests
type QueryKeygenBlock struct {
	KeygenBlock KeygenBlock `json:"keygen_block"`
	Signature   string      `json:"signature"`
}

// implement fmt.Stringer
func (n QueryKeygenBlock) String() string {
	return n.KeygenBlock.String()
}

type QueryKeysign struct {
	Keysign   TxOut  `json:"keysign"`
	Signature string `json:"signature"`
}

type QueryYggdrasilVaults struct {
	Vault      Vault       `json:"vault"`
	Status     NodeStatus  `json:"status"`
	Bond       cosmos.Uint `json:"bond"`
	TotalValue cosmos.Uint `json:"total_value"`
}

type QueryNodeAccount struct {
	NodeAddress         cosmos.AccAddress `json:"node_address"`
	Status              NodeStatus        `json:"status"`
	PubKeySet           common.PubKeySet  `json:"pub_key_set"`
	ValidatorConsPubKey string            `json:"validator_cons_pub_key"`
	Bond                cosmos.Uint       `json:"bond"`
	ActiveBlockHeight   int64             `json:"active_block_height"`
	BondAddress         common.Address    `json:"bond_address"`
	StatusSince         int64             `json:"status_since"`
	SignerMembership    common.PubKeys    `json:"signer_membership"`
	RequestedToLeave    bool              `json:"requested_to_leave"`
	ForcedToLeave       bool              `json:"forced_to_leave"`
	LeaveHeight         int64             `json:"leave_height"`
	IPAddress           string            `json:"ip_address"`
	Version             semver.Version    `json:"version"`
	SlashPoints         int64             `json:"slash_points"`
	Jail                Jail              `json:"jail"`
}

func NewQueryNodeAccount(na NodeAccount) QueryNodeAccount {
	return QueryNodeAccount{
		NodeAddress:         na.NodeAddress,
		Status:              na.Status,
		PubKeySet:           na.PubKeySet,
		ValidatorConsPubKey: na.ValidatorConsPubKey,
		Bond:                na.Bond,
		ActiveBlockHeight:   na.ActiveBlockHeight,
		BondAddress:         na.BondAddress,
		StatusSince:         na.StatusSince,
		SignerMembership:    na.SignerMembership,
		RequestedToLeave:    na.RequestedToLeave,
		ForcedToLeave:       na.ForcedToLeave,
		LeaveHeight:         na.LeaveHeight,
		IPAddress:           na.IPAddress,
		Version:             na.Version,
	}
}
