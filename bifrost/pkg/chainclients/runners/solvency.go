package runners

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/thorchain/thornode/common"
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
	timeout time.Duration,
	stopper <-chan struct{},
	wg *sync.WaitGroup) {
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
		case <-time.After(timeout):
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
