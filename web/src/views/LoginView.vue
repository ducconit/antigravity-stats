<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { serverState, testConnection } from '../stores/server';

const route = useRoute();
const router = useRouter();

const endpoint = ref(serverState.endpoint || 'http://localhost:58763');
const token = ref(serverState.token || '');
const showTokenField = ref(serverState.authRequired || !!token.value);
const isConnecting = ref(false);
const localError = ref('');
const localWarning = ref('');

const isRemoteHost = typeof window !== 'undefined' &&
  window.location.hostname !== 'localhost' &&
  window.location.hostname !== '127.0.0.1';

async function handleConnect() {
  localError.value = '';
  serverState.error = '';
  localWarning.value = '';
  isConnecting.value = true;

  try {
    const res = await testConnection(endpoint.value, token.value);
    if (res.success) {
      router.push({ name: 'dashboard' });
    } else {
      token.value = '';
      if (res.authRequired) {
        showTokenField.value = true;
        localWarning.value = res.error || 'Máy chủ yêu cầu Token/Mật khẩu xác thực.';
      } else {
        localError.value = res.error || 'Không thể kết nối đến máy chủ.';
      }
    }
  } finally {
    isConnecting.value = false;
  }
}

onMounted(async () => {
  // 1. Read URL query parameters (?host=... / ?endpoint=... and ?token=...)
  const queryHost = (route.query.host as string) || (route.query.endpoint as string);
  const queryToken = route.query.token as string;

  if (queryHost) {
    endpoint.value = queryHost.trim();
  }
  if (queryToken !== undefined && queryToken !== null) {
    token.value = queryToken.trim();
    showTokenField.value = true;
  }

  // 2. Only auto-connect and bypass if query parameter ?token or ?host is provided
  if (queryToken !== undefined || queryHost) {
    isConnecting.value = true;
    try {
      const res = await testConnection(endpoint.value, token.value);
      if (res.success) {
        router.push({ name: 'dashboard' });
      } else if (res.authRequired) {
        showTokenField.value = true;
        if (token.value) {
          localError.value = 'Token từ URL không hợp lệ hoặc đã hết hạn!';
        } else {
          localWarning.value = 'Máy chủ yêu cầu Token/Mật khẩu xác thực. Vui lòng nhập token bên dưới.';
        }
      } else {
        localError.value = res.error || 'Không thể kết nối đến máy chủ.';
      }
    } finally {
      isConnecting.value = false;
    }
  }
});
</script>

<template>
  <div class="min-h-screen bg-paper flex flex-col justify-center items-center px-4 py-12 sm:px-6 lg:px-8 selection:bg-cool selection:text-paper">
    <!-- Background subtle ambient glow -->
    <div class="absolute inset-0 overflow-hidden pointer-events-none">
      <div class="absolute -top-40 left-1/2 -translate-x-1/2 w-[600px] h-[350px] bg-cool/5 rounded-full blur-3xl"></div>
      <div class="absolute -bottom-40 left-1/2 -translate-x-1/2 w-[500px] h-[300px] bg-hit/5 rounded-full blur-3xl"></div>
    </div>

    <div class="relative w-full max-w-md space-y-8">
      <!-- Brand & Mascot Logo -->
      <div class="text-center space-y-3">
        <div class="inline-flex items-center justify-center p-3 rounded-2xl bg-paper-card border border-rule shadow-lg mb-1 group">
          <svg class="w-10 h-10 text-cool group-hover:scale-110 transition-transform duration-300" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
            <rect x="4" y="6" width="16" height="12" rx="3" stroke-width="2"/>
            <line x1="12" y1="2" x2="12" y2="6" stroke-width="2" stroke-linecap="round"/>
            <circle cx="12" cy="2" r="1" fill="currentColor"/>
            <circle cx="9" cy="11" r="1.5" fill="#10b981"/>
            <circle cx="15" cy="11" r="1.5" fill="#10b981"/>
            <path d="M9 15h6" stroke-width="1.8" stroke-linecap="round"/>
            <line x1="1" y1="12" x2="4" y2="12" stroke-width="2" stroke-linecap="round"/>
            <line x1="20" y1="12" x2="23" y2="12" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </div>

        <div class="flex items-center justify-center gap-2">
          <h1 class="text-2xl font-bold tracking-tight text-ink">
            Antigravity Stats
          </h1>
          <span class="text-xs badge-accent px-2 py-0.5 rounded-full font-mono font-semibold">
            v1.1
          </span>
        </div>
        <p class="text-xs text-ink-muted">
          Trực quan hóa Tiêu thụ Token, Cache Hit &amp; Chi phí Google Antigravity
        </p>
      </div>

      <!-- Main Connect Form Card -->
      <div class="bg-paper-card border border-rule rounded-2xl p-6 sm:p-8 shadow-xl space-y-6">
        <div class="border-b border-rule pb-4">
          <h2 class="text-base font-semibold text-ink flex items-center gap-2">
            <span class="w-2.5 h-2.5 rounded-full bg-hit animate-pulse"></span>
            Kết nối máy chủ cục bộ
          </h2>
          <p class="text-xs text-ink-dim mt-1">
            Giao diện kết nối trực tiếp đến máy chủ Go đang chạy trên máy tính của bạn.
          </p>
        </div>

        <!-- Alert messages -->
        <div
          v-if="localError || serverState.error"
          class="p-3.5 rounded-xl bg-red-950/40 border border-red-800/50 text-red-300 text-xs font-mono leading-relaxed"
        >
          ✕ {{ localError || serverState.error }}
        </div>

        <!-- Browser CORS / Local Network Access Helper (When on GitHub Pages or remote host) -->
        <div
          v-if="(localError || serverState.error) && isRemoteHost && (endpoint.includes('localhost') || endpoint.includes('127.0.0.1'))"
          class="p-4 rounded-xl bg-amber-950/30 border border-amber-800/50 text-amber-200/90 text-xs space-y-2.5"
        >
          <div class="font-bold flex items-center gap-2 text-amber-300">
            <span>🛡️</span>
            <span>Khắc phục lỗi CORS / Quyền Mạng cục bộ (Chrome/Edge):</span>
          </div>
          <div class="space-y-2 pl-1 leading-relaxed text-[11px]">
            <div class="p-2.5 rounded-lg bg-paper border border-rule font-sans">
              <strong class="text-cool block mb-0.5">💡 Cách 1 (Khuyên dùng - Nhanh nhất):</strong>
              Mở trực tiếp Web Dashboard trên máy tính tại:
              <a :href="endpoint" target="_blank" class="underline text-hit font-mono font-bold hover:text-cool ml-1">{{ endpoint }}</a>
              <span class="text-ink-dim block mt-0.5">(Chạy cùng nguồn same-origin, không bị trình duyệt chặn).</span>
            </div>
            <div class="p-2.5 rounded-lg bg-paper border border-rule font-sans">
              <strong class="text-accent block mb-0.5">💡 Cách 2 (Bật quyền trên GitHub Pages):</strong>
              Bấm biểu tượng <strong>Cài đặt trang</strong> (hình ổ khóa / công tắc bên trái thanh URL trình duyệt)
              &rarr; <strong>Cài đặt trang web (Site settings)</strong>
              &rarr; Tìm mục <strong>"Quyền truy cập mạng cục bộ" (Local network access)</strong>
              &rarr; Chọn <strong>Cho phép (Allow)</strong> rồi tải lại trang.
            </div>
          </div>
        </div>

        <div
          v-if="localWarning"
          class="p-3.5 rounded-xl bg-amber-950/40 border border-amber-800/50 text-amber-300 text-xs font-mono leading-relaxed"
        >
          ⚠ {{ localWarning }}
        </div>

        <form @submit.prevent="handleConnect" class="space-y-4">
          <!-- Endpoint Input -->
          <div>
            <label class="block text-xs font-semibold text-ink-muted mb-1.5 font-mono">
              Địa chỉ máy chủ (Server Endpoint)
            </label>
            <div class="relative">
              <input
                v-model="endpoint"
                type="text"
                required
                placeholder="http://localhost:58763"
                class="w-full px-3.5 py-2.5 rounded-xl bg-paper border border-rule text-ink placeholder-ink-dim text-xs font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool transition"
              />
            </div>
            <p class="text-[11px] text-ink-dim mt-1">
              Mặc định: <code class="text-cool font-mono">http://localhost:58763</code>
            </p>
          </div>

          <!-- Token Input (Conditional / Toggleable) -->
          <div v-if="showTokenField" class="animate-fade-in">
            <label class="block text-xs font-semibold text-ink-muted mb-1.5 font-mono">
              Mật khẩu / API Token (Bảo mật)
            </label>
            <input
              v-model="token"
              type="password"
              placeholder="Nhập secret token đã cấu hình với --token"
              class="w-full px-3.5 py-2.5 rounded-xl bg-paper border border-rule text-ink placeholder-ink-dim text-xs font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool transition"
            />
          </div>

          <div v-else class="text-right">
            <button
              type="button"
              @click="showTokenField = true"
              class="text-[11px] text-cool hover:underline font-mono"
            >
              + Nhập Token bảo mật nếu có
            </button>
          </div>

          <!-- Submit Button -->
          <button
            type="submit"
            :disabled="isConnecting"
            class="w-full mt-2 py-2.5 px-4 rounded-xl bg-cool hover:bg-cyan-400 text-paper font-semibold text-xs transition duration-200 flex items-center justify-center gap-2 shadow-lg disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <svg
              v-if="isConnecting"
              class="animate-spin w-4 h-4 text-paper"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span v-if="isConnecting">Đang kiểm tra kết nối...</span>
            <span v-else>Kết nối &amp; Vào Dashboard &rarr;</span>
          </button>
        </form>
      </div>

      <!-- Quick Run Instructions Footer -->
      <div class="bg-paper-card/60 border border-rule-subtle rounded-xl p-4 text-xs font-mono space-y-2 text-ink-muted">
        <div class="font-bold text-ink text-[11px] uppercase tracking-wider">
          🛠️ Hướng dẫn khởi chạy Server Go
        </div>
        <p class="text-[11px] text-ink-dim">
          Mở terminal trên máy tính và chạy lệnh sau để phục vụ dữ liệu:
        </p>
        <div class="bg-paper p-2.5 rounded-lg border border-rule text-cool select-all">
          ag-stats serve --port 58763
        </div>
        <div class="flex items-center justify-between text-[11px] text-ink-dim pt-1">
          <span>Kèm mật khẩu: <code class="text-accent">--token &lt;secret&gt;</code></span>
          <span>Tác giả: <strong class="text-cool">DNT</strong></span>
        </div>
      </div>
    </div>
  </div>
</template>
