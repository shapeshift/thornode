package types

import (
	"sort"

	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
)

// NodeTssTime
type NodeTssTime struct {
	Address cosmos.AccAddress `json:"address"`
	TssTime int64             `json:"tss_time"`
}

// TssMetric is a struct to store Tss Keygen metrics
type TssKeygenMetric struct {
	PubKey       common.PubKey `json:"pub_key"`
	NodeTssTimes []NodeTssTime `json:"node_tss_times"`
}

// NewTssKeygenMetric create a new instance of TssKeygenMetric
func NewTssKeygenMetric(pubkey common.PubKey) *TssKeygenMetric {
	return &TssKeygenMetric{PubKey: pubkey}
}

// AddNodeTssTime add node tss time
func (m *TssKeygenMetric) AddNodeTssTime(addr cosmos.AccAddress, keygenTime int64) {
	for _, item := range m.NodeTssTimes {
		if item.Address.Equals(addr) {
			item.TssTime = keygenTime
			return
		}
	}
	m.NodeTssTimes = append(m.NodeTssTimes, NodeTssTime{Address: addr, TssTime: keygenTime})
}

// GetMedianTime return the median time
func (m *TssKeygenMetric) GetMedianTime() int64 {
	return getMedianTime(m.NodeTssTimes)
}

// TssKeysignMetric is a struct to store Tss keysign metrics
type TssKeysignMetric struct {
	TxID         common.TxID   `json:"tx_id"`
	NodeTssTimes []NodeTssTime `json:"node_tss_times"`
}

// NewTssKeysignMetric create a new instance of TssKeysignMetric
func NewTssKeysignMetric(txID common.TxID) *TssKeysignMetric {
	return &TssKeysignMetric{
		TxID: txID,
	}
}

// AddNodeTssTime add node tss time
func (m *TssKeysignMetric) AddNodeTssTime(addr cosmos.AccAddress, keygenTime int64) {
	for _, item := range m.NodeTssTimes {
		if item.Address.Equals(addr) {
			item.TssTime = keygenTime
			return
		}
	}
	m.NodeTssTimes = append(m.NodeTssTimes, NodeTssTime{Address: addr, TssTime: keygenTime})
}

func getMedianTime(nodeTssTimes []NodeTssTime) int64 {
	sort.SliceStable(nodeTssTimes, func(i, j int) bool {
		return nodeTssTimes[i].TssTime < nodeTssTimes[j].TssTime
	})
	totalLen := len(nodeTssTimes)

	mid := len(nodeTssTimes) / 2
	if totalLen%2 != 0 {
		return nodeTssTimes[mid].TssTime
	}
	return (nodeTssTimes[mid-1].TssTime + nodeTssTimes[mid].TssTime) / 2
}

// GetMedianTime return median time
func (m *TssKeysignMetric) GetMedianTime() int64 {
	return getMedianTime(m.NodeTssTimes)
}
