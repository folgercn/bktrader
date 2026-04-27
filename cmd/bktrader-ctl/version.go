package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示 bktrader-ctl 构建版本 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		if outputJSON {
			fmt.Printf(`{"version":%q,"commit":%q,"buildDate":%q}`+"\n", Version, Commit, BuildDate)
			return nil
		}
		fmt.Printf("Version: %s\nCommit: %s\nBuildDate: %s\n", Version, Commit, BuildDate)
		return nil
	},
}
