package cmd

import (
	"fmt"
	"os"

	"antigravity-stats/internal/analyzer"

	"github.com/spf13/cobra"
)

var (
	dataDir     string
	pricingSpec string
)

// RootCmd is the base command for the antigravity-stats CLI.
var RootCmd = &cobra.Command{
	Use:     "ag-stats",
	Version: "1.0.0",
	Short:   "Antigravity Token & Cost Analytics Tool",
	Long: `⚡ Antigravity Token & Cost Visualizer
Phân tích và trực quan hóa số lượng token (Input, Cache Read, Output),
tỷ lệ Cache Hit, chi phí ước tính ($ / VNĐ) và Waterfall steps từ dữ liệu Antigravity.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if pricingSpec != "" {
			return analyzer.LoadPricingOverride(pricingSpec)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to running serve command if no subcommand provided
		return serveCmd.RunE(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Đường dẫn thư mục dữ liệu Antigravity (mặc định: ~/.gemini/antigravity)")
	RootCmd.PersistentFlags().StringVar(&pricingSpec, "pricing", "", "Tùy chỉnh bảng giá token (JSON file hoặc model tier: gemini-flash, gemini-pro, claude)")
}
