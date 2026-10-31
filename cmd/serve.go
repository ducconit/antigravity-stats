package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"antigravity-stats/internal/analyzer"
	"antigravity-stats/internal/web"

	"github.com/spf13/cobra"
)

var (
	servePort int
	serveHost string
	autoOpen  bool
	authToken string
	apiKey    string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Khởi chạy Web Dashboard trực quan hóa Token & Cost",
	Long:  `Khởi chạy máy chủ HTTP cục bộ và phục vụ giao diện Web Dashboard tương tác.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		effectiveToken := authToken
		if effectiveToken == "" {
			effectiveToken = apiKey
		}
		if effectiveToken == "" {
			if envTok := os.Getenv("AG_TOKEN"); envTok != "" {
				effectiveToken = envTok
			} else if envKey := os.Getenv("AG_API_KEY"); envKey != "" {
				effectiveToken = envKey
			}
		}

		scanner := analyzer.NewScanner(dataDir)
		server := web.NewServer(scanner, serveHost, servePort, effectiveToken)

		url := fmt.Sprintf("http://%s:%d", serveHost, servePort)
		dashboardURL := web.DefaultDashboardURL
		if autoOpen {
			go func() {
				time.Sleep(500 * time.Millisecond)
				openBrowser(dashboardURL)
			}()
		}

		fmt.Printf("🚀 Antigravity API Server đang chạy tại: %s\n", url)
		fmt.Printf("🌐 Web Dashboard (GitHub Pages): %s\n", dashboardURL)
		fmt.Printf("📂 Nguồn dữ liệu: %s\n", scanner.DataDir())
		if effectiveToken != "" {
			fmt.Printf("🔒 Chế độ xác thực: BẬT (API Key / Token: Yêu cầu Bearer Token hoặc X-API-Key)\n")
		} else {
			fmt.Printf("🔓 Chế độ xác thực: TẮT (Mặc định không yêu cầu API Key)\n")
		}
		fmt.Println("Nhấn Ctrl+C để dừng máy chủ.")

		return server.Start()
	},
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 58763, "Cổng lắng nghe của Web Server (mặc định: 58763)")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Địa chỉ host lắng nghe")
	serveCmd.Flags().BoolVarP(&autoOpen, "open", "o", false, "Tự động mở trình duyệt sau khi khởi động")
	serveCmd.Flags().StringVar(&authToken, "token", "", "Token/API Key bảo vệ API (mặc định: không yêu cầu)")
	serveCmd.Flags().StringVar(&apiKey, "api-key", "", "Bí danh của --token (API Key bảo vệ API)")
	RootCmd.AddCommand(serveCmd)
}
