package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
)

func init() {
	chartCmd.AddCommand(chartCandlesCmd)
	chartCmd.AddCommand(chartIndicatorsCmd)
	rootCmd.AddCommand(chartCmd)
}

var chartCmd = &cobra.Command{
	Use:   "chart",
	Short: "图表数据管理",
}

var chartCandlesCmd = &cobra.Command{
	Use:   "candles",
	Short: "获取 K 线数据 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		symbol, _ := cmd.Flags().GetString("symbol")
		res, _ := cmd.Flags().GetString("resolution")
		limit, _ := cmd.Flags().GetInt("limit")

		if symbol == "" {
			return fmt.Errorf("symbol is required")
		}

		v := url.Values{}
		v.Set("symbol", symbol)
		v.Set("resolution", res)
		v.Set("limit", fmt.Sprint(limit))

		client := getClient()
		resp, err := client.Request("GET", "/api/v1/chart/candles?"+v.Encode(), nil)
		handleResponse(resp, err)
		return nil
	},
}

var chartIndicatorsCmd = &cobra.Command{
	Use:   "indicators",
	Short: "获取技术指标数据 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		symbol, _ := cmd.Flags().GetString("symbol")
		res, _ := cmd.Flags().GetString("resolution")
		limit, _ := cmd.Flags().GetInt("limit")

		if symbol == "" {
			return fmt.Errorf("symbol is required")
		}

		v := url.Values{}
		v.Set("symbol", symbol)
		v.Set("resolution", res)
		v.Set("limit", fmt.Sprint(limit))

		client := getClient()
		resp, err := client.Request("GET", "/api/v1/chart/indicators?"+v.Encode(), nil)
		handleResponse(resp, err)
		return nil
	},
}

func init() {
	chartCandlesCmd.Flags().String("symbol", "", "交易对 (e.g. BTCUSDT)")
	chartCandlesCmd.Flags().String("resolution", "1h", "周期 (e.g. 1m, 5m, 1h, 1d)")
	chartCandlesCmd.Flags().Int("limit", 100, "返回数量")

	chartIndicatorsCmd.Flags().String("symbol", "", "交易对")
	chartIndicatorsCmd.Flags().String("resolution", "1h", "周期")
	chartIndicatorsCmd.Flags().Int("limit", 100, "返回数量")
}
