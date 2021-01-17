package cli

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"

	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

type ver struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
}

func (v ver) String() string {
	return v.Version
}

func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the THORChain module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(GetCmdGetVersion())
	return cmd
}

// GetCmdGetVersion queries current version
func GetCmdGetVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Gets the THORChain version and build information",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}
			clientCtx.OutputFormat = "json"

			out := ver{
				Version:   constants.SWVersion.String(),
				GitCommit: constants.GitCommit,
				BuildTime: constants.BuildTime,
			}
			return clientCtx.PrintObjectLegacy(out)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
