package runners

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/constants"
)

// SolvencyCheckProvider methods that a SolvencyChecker implementation should have
type SolvencyCheckProvider interface {
	GetHeight() (int64, error)
	ShouldReportSolvency(height int64) bool
	ReportSolvency(height int64) error
}

// SolvencyCheckRunner when a chain get marked as insolvent , and then get halt automatically , the chain client will stop scanning blocks , as a result , solvency checker will
// not report current solvency status to THORNode anymore, this method is to ensure that the chain client will continue to do solvency check even when the chain has been halted
func SolvencyCheckRunner(chain common.Chain,
	provider SolvencyCheckProvider,
	bridge *thorclient.ThorchainBridge,
	stopper <-chan struct{},
	wg *sync.WaitGroup,
) {
	logger := log.Logger.With().Str("chain", chain.String()).Logger()
	logger.Info().Msg("start solvency check runner")
	defer func() {
		wg.Done()
		logger.Info().Msg("finish  solvency check runner")
	}()
	if provider == nil {
		logger.Error().Msg("solvency checker provider is nil")
		return
	}
	for {
		select {
		case <-stopper:
			return
		case <-time.After(constants.ThorchainBlockTime):
			// check whether the chain is halted via mimir or not
			haltHeight, err := bridge.GetMimir(fmt.Sprintf("Halt%sChain", chain))
			if err != nil {
				logger.Err(err).Msg("fail to get chain halt height")
				continue
			}
			// when HaltHeight == 1 means admin halt the chain , no need to do solvency check
			// when Chain is not halted , the normal chain client will report solvency when it need to
			if haltHeight <= 1 {
				continue
			}

			// check whether the chain is halted via solvency check
			solvencyHaltHeight, err := bridge.GetMimir(fmt.Sprintf("SolvencyHalt%sChain", chain))
			if err != nil {
				logger.Err(err).Msg("fail to get solvency halt height")
				continue
			}
			// If SolvencyHalt<bnb>Chain <= 0, the chain is not halted, so the chain client will report solvency
			// If SolvencyHalt<bnb>Chain > 0 the chain has been halted via solvency, so we should report solvency here
			if solvencyHaltHeight <= 0 {
				continue
			}

			currentBlockHeight, err := provider.GetHeight()
			if err != nil {
				logger.Err(err).Msg("fail to get current block height")
				break
			}
			if provider.ShouldReportSolvency(currentBlockHeight) {
				logger.Info().Msgf("current block height: %d, report solvency again", currentBlockHeight)
				if err := provider.ReportSolvency(currentBlockHeight); err != nil {
					logger.Err(err).Msg("fail to report solvency")
				}
			}
		}
	}
}
