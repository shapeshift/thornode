package thorchain

import (
	"fmt"

	"github.com/blang/semver"
)

type YggdrasilFundMemo struct {
	MemoBase
	BlockHeight int64
}

func (m YggdrasilFundMemo) String() string {
	return fmt.Sprintf("YGGDRASIL+:%d", m.BlockHeight)
}

func (m YggdrasilFundMemo) GetBlockHeight() int64 {
	return m.BlockHeight
}

type YggdrasilReturnMemo struct {
	MemoBase
	BlockHeight int64
}

func (m YggdrasilReturnMemo) String() string {
	return fmt.Sprintf("YGGDRASIL-:%d", m.BlockHeight)
}

func (m YggdrasilReturnMemo) GetBlockHeight() int64 {
	return m.BlockHeight
}

func NewYggdrasilFund(blockHeight int64) YggdrasilFundMemo {
	return YggdrasilFundMemo{
		MemoBase:    MemoBase{TxType: TxYggdrasilFund},
		BlockHeight: blockHeight,
	}
}

func NewYggdrasilReturn(blockHeight int64) YggdrasilReturnMemo {
	return YggdrasilReturnMemo{
		MemoBase:    MemoBase{TxType: TxYggdrasilReturn},
		BlockHeight: blockHeight,
	}
}

func (p *parser) ParseYggdrasilFundMemo() (YggdrasilFundMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.116.0")):
		return p.ParseYggdrasilFundMemoV116()
	default:
		return ParseYggdrasilFundMemoV1(p.parts)
	}
}

func (p *parser) ParseYggdrasilReturnMemo() (YggdrasilReturnMemo, error) {
	switch {
	case p.version.GTE(semver.MustParse("1.116.0")):
		return p.ParseYggdrasilReturnMemoV116()
	default:
		return ParseYggdrasilReturnMemoV1(p.parts)
	}
}

func (p *parser) ParseYggdrasilFundMemoV116() (YggdrasilFundMemo, error) {
	blockHeight := p.getInt64(1, true, 0)
	return NewYggdrasilFund(blockHeight), p.Error()
}

func (p *parser) ParseYggdrasilReturnMemoV116() (YggdrasilReturnMemo, error) {
	blockHeight := p.getInt64(1, true, 0)
	return NewYggdrasilReturn(blockHeight), p.Error()
}
