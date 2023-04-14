package avalanche

import (
	"strings"

	ecore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
)

func isAcceptableError(err error) bool {
	return err == nil || err.Error() == txpool.ErrAlreadyKnown.Error() || strings.HasPrefix(err.Error(), ecore.ErrNonceTooLow.Error())
}
