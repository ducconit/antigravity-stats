<div align="center">

<img src="assets/banner.png" alt="Antigravity Token & Cost Visualizer Banner" width="100%" style="border-radius: 12px; max-width: 1100px; box-shadow: 0 10px 30px rgba(0,0,0,0.5);" />

# ⚡ Antigravity Token & Cost Visualizer (`ag-stats`)

**Công cụ dòng lệnh (CLI) và Web Dashboard phân tích số lượng Token, tỷ lệ Cache Hit, Chi phí LLM ($ & VNĐ) và Sơ đồ Waterfall Steps trong Google Antigravity.**

[![Release](https://img.shields.io/badge/Release-v1.1.0-6366F1?style=for-the-badge&logo=git)](https://github.com/ducconit/antigravity-stats/releases/latest)
[![Web Dashboard](https://img.shields.io/badge/Web_Client-GitHub_Pages-10B981?style=for-the-badge&logo=githubpages)](https://ducconit.github.io/antigravity-stats/)
[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Cobra](https://img.shields.io/badge/CLI-Cobra-38BDF8?style=for-the-badge&logo=gnubash)](https://github.com/spf13/cobra)
[![SQLite](https://img.shields.io/badge/Database-Pure_Go_SQLite-A78BFA?style=for-the-badge&logo=sqlite)](https://modernc.org/sqlite)
[![Author](https://img.shields.io/badge/Author-DNT-6366F1?style=for-the-badge&logo=github)](https://github.com/ducconit)
[![License](https://img.shields.io/badge/License-MIT-emerald?style=for-the-badge)](LICENSE)

[📦 Cài đặt & Tải Server](#-cài-đặt--khởi-chạy) • [🌐 Mở Web Dashboard (GitHub Pages)](https://ducconit.github.io/antigravity-stats/) • [🌳 Cấu trúc Phân cấp](#-kiến-trúc-phân-cấp-3-tầng) • [💻 Xem trên Terminal](#-xem-báo-cáo-trên-terminal-cli) • [👨‍💻 Tác giả](#-tác-giả)

---

</div>

## 📖 Giới thiệu

Trong quá trình làm việc với **Google Antigravity**, mỗi câu lệnh prompt, mỗi lần agent gọi công cụ (`run_command`, `read_file`, MCP tools) và quá trình Chain-of-Thought (CoT) thinking đều tiêu tốn một lượng token nhất định. Nhờ cơ chế **Context Caching (KV Cache)** của cụm máy chủ Gemini, chi phí và độ trễ được tối ưu đáng kể.

Tuy nhiên, người dùng khó có thể theo dõi trực tiếp:
- *Mỗi dự án đã ngốn bao nhiêu token?*
- *Hội thoại nào tiêu tốn nhiều tiền nhất?*
- *Tỷ lệ Cache Hit qua các step chuyển biến ra sao (từ Cache Miss ở Turn 1 sang tái sử dụng Cache ở các Turn sau)?*
- *Những model nào (`Gemini 3.8 Flash`, `Claude Opus 4.6`, `GPT-OSS 120B`) đã được sử dụng?*

**`ag-stats`** được xây dựng để giải quyết triệt để bài toán này — mang lại một giải pháp thống kê **hiệu năng cao, an toàn tuyệt đối và trực quan hóa sinh động**.

---

## 🚀 Tính năng nổi bật

### 1. 🌳 Cấu trúc phân cấp 3 tầng: Dự án &rarr; Hội thoại &rarr; Model
Hệ thống tự động liên kết dữ liệu từ `conversations_fts`, `conversation_summaries.db` và nhật ký `transcript.jsonl`:
- **Tầng 1 - Dự án (Project Accordion)**: Gom nhóm theo Workspace (`agent-pilot-extension`, `plane-api`, `statistic`,...). Thể hiện tổng tokens, tỷ lệ Cache Hit của cả dự án, tổng chi phí ($ và VNĐ).
- **Tầng 2 - Hội thoại (Conversation List)**: Liệt kê chi tiết từng phiên chat thuộc dự án, thời gian, số step và token tiêu tốn.
- **Tầng 3 - Nhóm Model trong từng hội thoại**: Bấm nút **`🤖 Model`** để xem bảng bóc tách chi tiết từng model đã tham gia xử lý (kể cả khi bạn chuyển đổi model giữa chừng trong cùng một phiên chat).

### 2. ⚡ Step Waterfall Visualizer
Click vào bất kỳ hội thoại nào để mở sơ đồ dòng thời gian trực quan:
- Hiển thị từng bước (`User Request` &rarr; `Tool Exec` &rarr; `LLM Response`).
- Minh chứng rõ ràng cơ chế **Cache Hit**: Turn 1 nạp prefix (Cache Miss 0%) $\to$ Turn 2 trở đi tái sử dụng bộ nhớ đệm (Cache Hit > 85-95%).
- Hiển thị thông tin công cụ đã gọi, thời gian thực thi, CoT thinking preview và badge định danh Model tương ứng.

### 3. 🌐 Mô hình Phân tách: Web Client (GitHub Pages) + Self-Hosted CLI Server
Hỗ trợ cả 2 mô hình linh hoạt:
- **Cloud Client (GitHub Pages)**: Truy cập trực tiếp Web Dashboard được host online trên GitHub Pages mà không cần build mã nguồn hay cài đặt Node.js.
- **Self-Hosted CLI Server**: Chỉ cần tải binary Go về máy tính và chạy lệnh `ag-stats serve`.
- **Handshake API (`/api/config`)**: Khi mở Web Client, giao diện tự động bắt tay với server Go tại `http://localhost:58763` (hoặc endpoint tuỳ chọn).
- **CORS & Private Network Access (PNA)**: Server tích hợp sẵn CORS và Chromium PNA header (`Access-Control-Allow-Private-Network`), đảm bảo Chrome/Edge/Firefox không chặn request từ trang HTTPS của GitHub Pages về `localhost`.
- **Tùy chọn Bảo mật (Optional Auth)**: Mặc định không cần auth. Nếu muốn mở server ra mạng LAN hoặc VPS, bạn có thể truyền cờ `--token <secret>` để kích hoạt xác thực Bearer token an toàn.
- **Tối ưu hóa Binary Siêu Nhẹ (~12 MB)**: Server tách biệt hoàn toàn phần giao diện web sang GitHub Pages, loại bỏ việc nhúng HTML/CSS/JS nặng nề vào mã nguồn Go. Binary được biên dịch với cờ tối ưu hóa chuyên sâu `-trimpath -ldflags="-s -w"` giúp loại bỏ toàn bộ DWARF debug tables, symbol tables và đường dẫn máy build.

### 4. ⚡ Siêu tốc & An toàn tuyệt đối
- **Worker Pool song song**: Quét hơn **314+ sessions** với hơn **95.000 steps** trong chưa đầy **0.5 giây**.
- **Pure-Go SQLite** (`modernc.org/sqlite`): Không cần GCC / CGO, dễ dàng build và chạy trên mọi hệ điều hành.
- **Read-Only Mode**: Truy cập SQLite và files log ở chế độ chỉ đọc (`?mode=ro`), bảo vệ toàn vẹn 100% dữ liệu gốc của Antigravity.
- **Bảo mật XSS**: Toàn bộ dữ liệu hiển thị trên Web được chuẩn hóa qua bộ lọc HTML entity escaping.

---

## 🛠️ Cài đặt & Khởi chạy

Bạn có thể sử dụng `ag-stats` theo 2 cách cực kỳ đơn giản:

### Cách 1: Tải nhanh Binary dựng sẵn từ GitHub Releases (Khuyên dùng - Không cần cài Go)

Truy cập trang **[GitHub Releases v1.1.0](https://github.com/ducconit/antigravity-stats/releases/latest)** và tải file thực thi phù hợp với hệ điều hành của bạn:

| Hệ điều hành | File tải về | Lệnh chạy nhanh trên Terminal |
| :--- | :--- | :--- |
| **Linux (x86_64)** | [`ag-stats-linux-amd64`](https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-linux-amd64) | `chmod +x ag-stats-linux-amd64 && ./ag-stats-linux-amd64 serve` |
| **Linux (ARM64)** | [`ag-stats-linux-arm64`](https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-linux-arm64) | `chmod +x ag-stats-linux-arm64 && ./ag-stats-linux-arm64 serve` |
| **macOS (Apple Silicon M1-M4)** | [`ag-stats-darwin-arm64`](https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-darwin-arm64) | `chmod +x ag-stats-darwin-arm64 && ./ag-stats-darwin-arm64 serve` |
| **macOS (Intel)** | [`ag-stats-darwin-amd64`](https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-darwin-amd64) | `chmod +x ag-stats-darwin-amd64 && ./ag-stats-darwin-amd64 serve` |
| **Windows (x64)** | [`ag-stats-windows-amd64.exe`](https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-windows-amd64.exe) | Chạy file `ag-stats-windows-amd64.exe serve` trong PowerShell / CMD |

> **💡 Cài nhanh 1 dòng trên Linux / macOS:**
> ```bash
> # Linux (x86_64)
> curl -sL https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-linux-amd64 -o ag-stats && chmod +x ag-stats && ./ag-stats serve
> 
> # macOS Apple Silicon
> curl -sL https://github.com/ducconit/antigravity-stats/releases/latest/download/ag-stats-darwin-arm64 -o ag-stats && chmod +x ag-stats && ./ag-stats serve
> ```

---

### Cách 2: Tự biên dịch từ mã nguồn (Dành cho Lập trình viên)

Yêu cầu máy tính đã cài đặt **Golang 1.22+**:

```bash
# 1. Clone repository
git clone https://github.com/ducconit/antigravity-stats.git
cd antigravity-stats

# 2. Biên dịch binary
make build
# Hoặc dùng lệnh go build trực tiếp:
go build -o bin/ag-stats main.go

# 3. Chạy máy chủ server
./bin/ag-stats serve
```

---

## 📖 Hướng dẫn Xem Dashboard & Kết nối

### 🌐 Bước 1: Khởi động Server trên máy tính
Chạy lệnh server từ binary bạn đã tải hoặc biên dịch:
```bash
# Khởi chạy server lắng nghe tại cổng mặc định 58763
./ag-stats serve

# Hoặc tùy chỉnh cổng nếu muốn:
./ag-stats serve --port 8088

# Kích hoạt chế độ bảo vệ API bằng Token (khi host trên VPS hoặc mạng LAN):
./ag-stats serve --token "matkhaubi_mat"

# Chỉ định đường dẫn thư mục Antigravity tùy biến (nếu dùng thư mục khác mặc định):
./ag-stats serve --data-dir ~/.gemini/antigravity
```

### 💻 Bước 2: Truy cập Web Client để xem Dashboard

#### 👉 Lựa chọn A: Mở trực tiếp Web Dashboard trên GitHub Pages (Khuyên dùng)
1. Mở trình duyệt và truy cập: **[https://ducconit.github.io/antigravity-stats/](https://ducconit.github.io/antigravity-stats/)**
2. Hộp thoại **Kết nối Máy chủ Antigravity** sẽ hiển thị với địa chỉ điền sẵn `http://localhost:58763`.
3. Bấm **Kết nối**:
   - Nếu server không bật auth: Tự động kết nối và mở Dashboard thống kê ngay lập tức.
   - Nếu server chạy kèm `--token`: Nhập token bảo vệ và xác nhận.
4. Trình duyệt tự động lưu endpoint & token vào `localStorage`. Trên Header sẽ hiện trạng thái `● localhost:58763 [Đổi]` để bạn linh hoạt chuyển đổi giữa các máy chủ bất kỳ lúc nào.

#### 👉 Lựa chọn B: Mở giao diện nhúng nội bộ (Offline không cần mạng)
Chỉ cần thêm cờ `-o` hoặc `--open` khi chạy server:
```bash
./ag-stats serve -o
```
Server sẽ tự động mở trình duyệt tại địa chỉ cục bộ: **[http://127.0.0.1:58763](http://127.0.0.1:58763)**.

---

### 2. Xem báo cáo trên Terminal (CLI)

```bash
# Xem tổng quan và danh sách dự án
./bin/ag-stats stats
# hoặc dùng alias:
./bin/ag-stats summary

# Tùy chỉnh số lượng phiên gần nhất hiển thị:
./bin/ag-stats stats --limit 20

# Tính toán chi phí theo mô hình khác (ví dụ: gemini-pro hoặc claude):
./bin/ag-stats stats --pricing gemini-pro --limit 5

# Xuất định dạng JSON cho CI/CD hoặc pipeline tự động:
./bin/ag-stats stats --json
```

**Ví dụ output trên Terminal:**
```
================================================================================
⚡ ANTIGRAVITY TOKEN & COST SUMMARY
================================================================================
📂 Thư mục quét:             314 phiên làm việc (75 phiên có token metrics)
🔢 Tổng số Steps:             95444
📊 Tổng số Tokens:            1649.10M tokens
📥 Input Tokens (Mới):        101.84M tokens
⚡ Cache Read Tokens (Hit):   1540.44M tokens
🎯 Tỷ lệ Cache Hit trung bình: 93.80%
📤 Output Tokens:             6.82M tokens
💵 Ước tính chi phí (USD):    $51.4230
🇻🇳 Ước tính chi phí (VNĐ):    ~1.31M VNĐ
================================================================================

📁 THỐNG KÊ THEO DỰ ÁN (PROJECTS SUMMARY):
Dự án                        | Hội thoại  | Tokens     | Cache %  | Cost ($)
-----------------------------+------------+------------+----------+---------
agent-pilot-extension        | 28         | 619.67M    |   94.3%  | $19.1755
plane-api                    | 8          | 501.69M    |   95.3%  | $14.9647
erpmanager                   | 1          | 131.61M    |   92.9%  | $4.1429 
lagom                        | 3          | 71.13M     |   90.5%  | $2.3605 
vietsub                      | 1          | 70.28M     |   95.4%  | $2.0644 
figma-cli                    | 1          | 69.04M     |   91.9%  | $2.2522 
statistic                    | 2          | 18.57M     |   92.8%  | $0.6113 
...                          | ...        | ...        |   ...    | ...
================================================================================
```

---

## 🏗️ Kiến trúc thư mục dự án

```
.
├── .github/
│   └── workflows/
│       └── deploy-pages.yml # Tự động deploy Web Client lên GitHub Pages
├── assets/
│   ├── banner.png       # Banner dự án (16:9 Digital Art)
│   ├── banner.svg       # Vector banner chất lượng cao
│   └── thumbnail.png    # Thumbnail / OG Image chia sẻ mạng xã hội
├── cmd/
│   ├── root.go          # Cobra Root Command, flags cấu hình (--data-dir, --pricing)
│   ├── serve.go         # Lệnh 'serve' với flags --port, --token, --open
│   └── stats.go         # Lệnh 'stats' (alias 'summary') hiển thị báo cáo terminal
├── docs/                # Giao diện Web Dashboard SPA tĩnh lưu trữ trên GitHub Pages
├── internal/
│   ├── analyzer/        # Module bóc tách dữ liệu transcript và tính toán chi phí
│   │   ├── types.go     # Structs: StepMetric, SessionMetric, ProjectStats, GlobalStats
│   │   ├── pricing.go   # Bảng giá quy đổi token các model & tỷ giá USD/VND
│   │   ├── scanner.go   # Worker Pool quét brain/ và sqlite database
│   │   └── scanner_test.go # Unit test cho scanner, rune truncation & pricing
│   └── web/
│       ├── server.go    # HTTP Handlers REST APIs, CORS, Private Network Access & Auth
│       └── server_test.go # Unit test toàn diện cho HTTP endpoints, CORS & Auth
├── Makefile             # Quản lý build, test, release cross-platform, clean
├── README.md            # Tài liệu dự án
├── go.mod
├── go.sum
└── main.go              # Entry point
```

---

## 👨‍💻 Tác giả

Được thiết kế và phát triển bởi **DNT**  
- **GitHub**: [@ducconit](https://github.com/ducconit)  
- **Email**: `ducconit@gmail.com`

---

## 📄 Bản quyền (License)

Dự án phát hành theo giấy phép [MIT License](LICENSE).
