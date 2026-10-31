package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"antigravity-stats/internal/analyzer"

	"github.com/spf13/cobra"
)

var (
	statsLimit  int
	outputJSON  bool
)

var statsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"summary"},
	Short:   "Xem báo cáo thống kê Token & Cost trên Terminal",
	Long:    `Quét dữ liệu các phiên làm việc và in bảng tổng kết Token, Cache Hit Rate và chi phí ngay trên CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		scanner := analyzer.NewScanner(dataDir)
		stats, sessions, err := scanner.ScanAll()
		if err != nil {
			return fmt.Errorf("không thể quét dữ liệu: %w", err)
		}

		if outputJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(stats)
		}

		fmt.Println("================================================================================")
		fmt.Println("⚡ ANTIGRAVITY TOKEN & COST SUMMARY")
		fmt.Println("================================================================================")
		fmt.Printf("📂 Thư mục quét:             %d phiên làm việc (%d phiên có token metrics)\n", stats.TotalSessions, stats.ActiveTokenSessions)
		fmt.Printf("🔢 Tổng số Steps:             %d\n", stats.TotalSteps)
		fmt.Printf("📊 Tổng số Tokens:            %s tokens\n", formatBigNum(stats.TotalTokens))
		fmt.Printf("📥 Input Tokens (Mới):        %s tokens\n", formatBigNum(stats.TotalInputTokens))
		fmt.Printf("⚡ Cache Read Tokens (Hit):   %s tokens\n", formatBigNum(stats.TotalCacheRead))
		fmt.Printf("🎯 Tỷ lệ Cache Hit trung bình: %.2f%%\n", stats.OverallCacheHitRate)
		fmt.Printf("📤 Output Tokens:             %s tokens\n", formatBigNum(stats.TotalOutput))
		fmt.Printf("💵 Ước tính chi phí (USD):    $%.4f\n", stats.TotalCostUSD)
		fmt.Printf("🇻🇳 Ước tính chi phí (VNĐ):    ~%s VNĐ\n", formatBigNum(int64(stats.TotalCostVND)))
		fmt.Println("================================================================================")

		// Print Project Summary
		if len(stats.Projects) > 0 {
			fmt.Println("\n📁 THỐNG KÊ THEO DỰ ÁN (PROJECTS SUMMARY):")
			fmt.Printf("%-28s | %-10s | %-10s | %-8s | %-8s\n", "Dự án", "Hội thoại", "Tokens", "Cache %", "Cost ($)")
			fmt.Println("-----------------------------+------------+------------+----------+---------")
			for _, p := range stats.Projects {
				if p.SessionCount == 0 {
					continue
				}
				name := p.Name
				if len([]rune(name)) > 26 {
					name = string([]rune(name)[:23]) + "..."
				}
				fmt.Printf("%-28s | %-10d | %-10s | %6.1f%%  | $%-7.4f\n",
					name, p.SessionCount, formatBigNum(p.TotalTokens), p.OverallCacheHitRate, p.TotalCostUSD)
			}
			fmt.Println("================================================================================")
		}

		limit := statsLimit
		if limit > len(sessions) {
			limit = len(sessions)
		}

		if limit > 0 {
			fmt.Printf("\n📋 DANH SÁCH %d PHIÊN LÀM VIỆC GẦN NHẤT:\n\n", limit)
			fmt.Printf("%-18s | %-32s | %-6s | %-10s | %-8s | %-8s\n", "Thời gian", "Tiêu đề / Session ID", "Steps", "Tokens", "Cache %", "Cost ($)")
			fmt.Println("-------------------+----------------------------------+--------+------------+----------+---------")
			for i := 0; i < limit; i++ {
				s := sessions[i]
				dateStr := "-"
				if !s.LastModified.IsZero() {
					dateStr = s.LastModified.Local().Format("2006-01-02 15:04")
				}
				title := s.Title
				if len(title) > 30 {
					title = title[:28] + ".."
				}
				fmt.Printf("%-18s | %-32s | %-6d | %-10s | %6.1f%%  | $%-7.4f\n",
					dateStr, title, s.StepCount, formatBigNum(s.TotalTokens), s.CacheHitRate, s.CostUSD)
			}
			fmt.Println("--------------------------------------------------------------------------------")
			fmt.Println("💡 Chạy lệnh 'ag-stats serve' để mở giao diện Web Dashboard tương tác & Waterfall visualizer.")
		}

		return nil
	},
}

func formatBigNum(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000.0)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000.0)
	}
	return fmt.Sprintf("%d", n)
}

func init() {
	statsCmd.Flags().IntVarP(&statsLimit, "limit", "n", 10, "Số lượng phiên gần nhất cần hiển thị")
	statsCmd.Flags().BoolVar(&outputJSON, "json", false, "Xuất kết quả dưới định dạng JSON")
	RootCmd.AddCommand(statsCmd)
}
