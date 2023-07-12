package thorchain

import (
	"errors"
	"fmt"
	"strconv"
)

func ParseYggdrasilFundMemoV1(parts []string) (YggdrasilFundMemo, error) {
	if len(parts) < 2 {
		return YggdrasilFundMemo{}, errors.New("not enough parameters")
	}
	blockHeight, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return YggdrasilFundMemo{}, fmt.Errorf("fail to convert (%s) to a valid block height: %w", parts[1], err)
	}
	return NewYggdrasilFund(blockHeight), nil
}

func ParseYggdrasilReturnMemoV1(parts []string) (YggdrasilReturnMemo, error) {
	if len(parts) < 2 {
		return YggdrasilReturnMemo{}, errors.New("not enough parameters")
	}
	blockHeight, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return YggdrasilReturnMemo{}, fmt.Errorf("fail to convert (%s) to a valid block height: %w", parts[1], err)
	}
	return NewYggdrasilReturn(blockHeight), nil
}
