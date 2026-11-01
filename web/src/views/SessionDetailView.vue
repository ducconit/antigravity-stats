<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { serverState, fetchSessionDetail, disconnect } from '../stores/server';
import type { SessionMetric, StepMetric } from '../types';

const route = useRoute();
const router = useRouter();

const sessionId = computed(() => (route.params.id as string) || '');
const session = ref<SessionMetric | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);

// UI states
const copied = ref(false);
const showFullKPIs = ref(false);
const stepSearchQuery = ref('');
const stepTypeFilter = ref<'all' | 'user' | 'model' | 'tool'>('all');
const expandedThinking = ref<Record<number, boolean>>({});
const allThinkingExpanded = ref(false);
const expandedSteps = ref<Record<number, boolean>>({});
const allDetailsExpanded = ref(false);

function toggleStepExpanded(stepIndex: number) {
  expandedSteps.value[stepIndex] = !expandedSteps.value[stepIndex];
}

function isStepExpanded(stepIndex: number): boolean {
  return !!expandedSteps.value[stepIndex];
}

function hasDetailedContent(step: StepMetric): boolean {
  if (!step.content) return false;
  const previewLen = step.content_preview?.length || 0;
  return step.content.length > previewLen || (step.content_preview?.endsWith('...') ?? false);
}

function toggleAllDetails() {
  allDetailsExpanded.value = !allDetailsExpanded.value;
  if (session.value?.steps) {
    for (const step of session.value.steps) {
      if (hasDetailedContent(step)) {
        expandedSteps.value[step.step_index] = allDetailsExpanded.value;
      }
    }
  }
}

// Chat Virtualization / Reverse Infinite Scroll
const CHUNK_SIZE = 25;
const visibleStepCount = ref(CHUNK_SIZE);
const chatContainerRef = ref<HTMLElement | null>(null);
const showScrollBottomBtn = ref(false);
const isLoadingEarlier = ref(false);

function formatNum(n?: number): string {
  if (n === undefined || n === null) return '0';
  if (n >= 1_000_000_000) return (n / 1_000_000_000).toFixed(2) + 'B';
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return n.toLocaleString('vi-VN');
}

function formatCurrencyUSD(n?: number): string {
  if (n === undefined || n === null) return '$0.0000';
  return `$${n.toFixed(4)}`;
}

function formatCurrencyVND(n?: number): string {
  if (n === undefined || n === null) return '~0 ₫';
  return `~${Math.round(n).toLocaleString('vi-VN')} ₫`;
}

function formatTime(iso?: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function cleanUserContent(text?: string): string {
  if (!text) return '';
  let cleaned = text;
  // If full tag is present
  const m = cleaned.match(/<USER_REQUEST>([\s\S]*?)<\/USER_REQUEST>/i);
  if (m && m[1]) {
    cleaned = m[1].trim();
  } else {
    // If only opening or truncated closing tag
    cleaned = cleaned.replace(/<USER_REQUEST>/gi, '');
    cleaned = cleaned.replace(/<\/USER_R[^>]*>?/gi, '');
    cleaned = cleaned.replace(/<\/USER_REQUEST>/gi, '');
  }
  // Remove user settings change blocks
  cleaned = cleaned.replace(/<USER_SETTINGS_CHANGE>[\s\S]*?<\/USER_SETTINGS_CHANGE>/gi, '');
  cleaned = cleaned.replace(/<USER_SETTINGS_CHANGE>[\s\S]*/gi, '');
  return cleaned.trim() || text;
}

async function loadSession() {
  if (!sessionId.value) {
    error.value = 'Không tìm thấy ID phiên làm việc.';
    loading.value = false;
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    const data = await fetchSessionDetail(sessionId.value);
    if (data) {
      session.value = data;
      visibleStepCount.value = CHUNK_SIZE;
      scrollToBottom(false);
    } else {
      if (!serverState.isConnected) {
        router.push({ name: 'login' });
        return;
      }
      error.value = `Không thể tìm thấy phiên làm việc có ID "${sessionId.value}".`;
    }
  } catch (err) {
    console.error('Lỗi khi tải chi tiết phiên:', err);
    error.value = 'Đã xảy ra lỗi khi tải dữ liệu từ máy chủ.';
  } finally {
    loading.value = false;
  }
}

function fallbackCopy(text: string) {
  try {
    const el = document.createElement('textarea');
    el.value = text;
    el.setAttribute('readonly', '');
    el.style.position = 'absolute';
    el.style.left = '-9999px';
    document.body.appendChild(el);
    el.select();
    document.execCommand('copy');
    document.body.removeChild(el);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch (e) {
    console.warn('Could not copy session id:', e);
  }
}

function copySessionId() {
  if (!session.value?.id) return;
  if (typeof navigator !== 'undefined' && navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard
      .writeText(session.value.id)
      .then(() => {
        copied.value = true;
        setTimeout(() => {
          copied.value = false;
        }, 2000);
      })
      .catch(() => {
        fallbackCopy(session.value?.id || '');
      });
  } else {
    fallbackCopy(session.value.id);
  }
}

function toggleThinking(stepIndex: number) {
  expandedThinking.value[stepIndex] = !expandedThinking.value[stepIndex];
}

function toggleAllThinking() {
  allThinkingExpanded.value = !allThinkingExpanded.value;
  if (session.value?.steps) {
    for (const step of session.value.steps) {
      if (step.thinking_preview) {
        expandedThinking.value[step.step_index] = allThinkingExpanded.value;
      }
    }
  }
}

function handleChangeServer() {
  disconnect();
  router.push({ name: 'login' });
}

// All Filtered Steps
const allFilteredSteps = computed(() => {
  if (!session.value?.steps) return [];
  const q = stepSearchQuery.value.trim().toLowerCase();
  const filter = stepTypeFilter.value;

  return session.value.steps.filter((step) => {
    // Type Filter
    if (filter === 'user' && step.source !== 'USER_EXPLICIT' && step.type !== 'USER_INPUT') return false;
    if (filter === 'model' && step.type !== 'PLANNER_RESPONSE' && step.source !== 'MODEL') return false;
    if (filter === 'tool' && (!step.tool_calls || step.tool_calls.length === 0)) return false;

    // Search Query
    if (!q) return true;
    const matchContent = step.content_preview?.toLowerCase().includes(q);
    const matchThinking = step.thinking_preview?.toLowerCase().includes(q);
    const matchTool = step.tool_calls?.some(
      (t) => t.name.toLowerCase().includes(q) || t.toolSummary?.toLowerCase().includes(q)
    );
    const matchModel = step.model_name?.toLowerCase().includes(q);

    return matchContent || matchThinking || matchTool || matchModel;
  });
});

// Sliced visible steps: takes the bottom/latest N steps
const visibleSteps = computed(() => {
  const list = allFilteredSteps.value;
  const count = visibleStepCount.value;
  if (list.length <= count) return list;
  return list.slice(list.length - count);
});

const hasEarlierSteps = computed(() => {
  return allFilteredSteps.value.length > visibleStepCount.value;
});

const remainingEarlierCount = computed(() => {
  return Math.max(0, allFilteredSteps.value.length - visibleStepCount.value);
});

// Auto-scroll mechanics
function scrollToBottom(smooth = true) {
  nextTick(() => {
    const el = chatContainerRef.value;
    if (!el) return;
    if (smooth) {
      el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
    } else {
      el.scrollTop = el.scrollHeight;
    }
  });
}

function loadEarlierSteps() {
  if (!hasEarlierSteps.value || isLoadingEarlier.value) return;
  isLoadingEarlier.value = true;

  const el = chatContainerRef.value;
  const prevScrollHeight = el ? el.scrollHeight : 0;
  const prevScrollTop = el ? el.scrollTop : 0;

  visibleStepCount.value = Math.min(
    allFilteredSteps.value.length,
    visibleStepCount.value + CHUNK_SIZE
  );

  nextTick(() => {
    if (el) {
      // Preserve scroll position relative to newly prepended items
      const newScrollHeight = el.scrollHeight;
      el.scrollTop = newScrollHeight - prevScrollHeight + prevScrollTop;
    }
    isLoadingEarlier.value = false;
  });
}

function handleScroll() {
  const el = chatContainerRef.value;
  if (!el) return;

  // 1. If near top (<= 150px) and earlier steps exist, auto-prepend older steps!
  if (el.scrollTop <= 150 && hasEarlierSteps.value && !isLoadingEarlier.value) {
    loadEarlierSteps();
  }

  // 2. Check if scrolled away from bottom (show button)
  const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
  showScrollBottomBtn.value = distanceFromBottom > 300;
}

// Watch filters to reset slice and scroll to bottom
watch([stepSearchQuery, stepTypeFilter], () => {
  visibleStepCount.value = CHUNK_SIZE;
  scrollToBottom(false);
});

// Immediate reactive watcher on sessionId to handle in-app route changes
watch(
  sessionId,
  (newId) => {
    if (newId) {
      stepSearchQuery.value = '';
      stepTypeFilter.value = 'all';
      expandedThinking.value = {};
      allThinkingExpanded.value = false;
      expandedSteps.value = {};
      allDetailsExpanded.value = false;
      loadSession();
    }
  },
  { immediate: true }
);
</script>

<template>
  <div class="h-screen bg-paper flex flex-col overflow-hidden">
    <!-- Header -->
    <header class="border-b border-rule bg-paper-card/80 backdrop-blur sticky top-0 z-30 flex-shrink-0">
      <div class="w-full px-4 sm:px-6 lg:px-8 py-3 flex items-center justify-between gap-4">
        <!-- Back Button & Brand -->
        <div class="flex items-center gap-3 min-w-0">
          <button
            @click="router.push({ name: 'dashboard' })"
            class="back-to-dashboard px-2.5 sm:px-3 py-1.5 rounded-lg bg-paper-subtle hover:bg-rule border border-rule text-xs font-mono font-medium text-ink transition flex items-center gap-1.5 group flex-shrink-0 cursor-pointer"
            title="Quay lại danh sách Dashboard"
          >
            <svg
              class="w-4 h-4 text-cool group-hover:-translate-x-0.5 transition-transform"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <line x1="19" y1="12" x2="5" y2="12"/>
              <polyline points="12 19 5 12 12 5"/>
            </svg>
            <span class="hidden sm:inline">Quay lại Dashboard</span>
            <span class="sm:hidden">Dashboard</span>
          </button>

          <div class="h-4 w-px bg-rule hidden sm:block"></div>

          <div class="min-w-0">
            <h1 class="font-bold text-sm sm:text-base text-ink truncate flex items-center gap-2">
              <span>Hội thoại:</span>
              <span v-if="session" class="text-cool truncate max-w-xs sm:max-w-md">
                {{ session.title || session.id.slice(0, 8) }}
              </span>
            </h1>
          </div>
        </div>

        <!-- Server Controls -->
        <div class="flex items-center gap-2.5 flex-shrink-0">
          <button
            @click="handleChangeServer"
            class="px-2.5 sm:px-3 py-1.5 rounded-lg bg-paper-card hover:bg-paper-subtle border border-rule text-xs font-medium text-ink transition flex items-center gap-1.5 shadow group"
            title="Bấm để đổi máy chủ hoặc đăng xuất"
          >
            <span class="w-2 h-2 rounded-full bg-hit animate-pulse"></span>
            <span class="font-mono text-cool group-hover:text-cyan-300">
              {{ serverState.endpoint.replace('http://', '') }}
            </span>
            <span class="text-[10px] text-ink-dim group-hover:text-ink">Đổi</span>
          </button>

          <button
            @click="loadSession"
            :disabled="loading"
            class="px-2.5 sm:px-3 py-1.5 rounded-lg bg-paper-card hover:bg-paper-subtle border border-rule text-xs font-medium text-ink transition flex items-center gap-1.5 shadow"
            title="Tải lại chi tiết phiên này"
          >
            <svg
              class="w-3.5 h-3.5 text-cool"
              :class="{ 'animate-spin': loading }"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
            </svg>
            <span class="hidden sm:inline">Làm mới</span>
          </button>
        </div>
      </div>
    </header>

    <!-- Main Content Container -->
    <main class="w-full px-3 sm:px-6 lg:px-8 py-3 flex-1 min-h-0 flex flex-col space-y-2.5 overflow-hidden">
      <!-- Loading Skeleton -->
      <div v-if="loading" class="flex-1 space-y-4 animate-pulse">
        <div class="h-20 bg-paper-card border border-rule rounded-xl"></div>
        <div class="h-[600px] bg-paper-card border border-rule rounded-2xl"></div>
      </div>

      <!-- Error State -->
      <div
        v-else-if="error"
        class="bg-rose-500/10 border border-rose-500/30 rounded-2xl p-8 text-center space-y-4 max-w-2xl mx-auto my-auto"
      >
        <div class="w-12 h-12 rounded-full bg-rose-500/20 text-rose-400 mx-auto flex items-center justify-center">
          <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"/>
            <line x1="12" y1="8" x2="12" y2="12"/>
            <line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
        </div>
        <div>
          <h2 class="text-base font-bold text-rose-300">{{ error }}</h2>
          <p class="text-xs text-ink-muted mt-1 font-mono">
            Vui lòng kiểm tra lại ID phiên làm việc hoặc đảm bảo file transcript.jsonl còn tồn tại trên máy chủ.
          </p>
        </div>
        <router-link
          to="/dashboard"
          class="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-paper-card hover:bg-paper-subtle border border-rule text-xs font-mono font-medium text-ink transition"
        >
          &larr; Về lại Dashboard
        </router-link>
      </div>

      <!-- Loaded Content -->
      <template v-else-if="session">
        <!-- Compact Session Bar & Quick Stats (Collapsible to maximize chat) -->
        <div class="bg-paper-card border border-rule rounded-xl p-3 sm:p-4 shadow-sm flex-shrink-0 transition">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-wrap items-center gap-2 min-w-0">
              <!-- Subagent Badge -->
              <span
                v-if="session.is_subagent"
                class="text-xs px-2.5 py-0.5 rounded font-mono font-semibold bg-purple-500/15 text-purple-300 border border-purple-500/30 flex items-center gap-1.5 flex-shrink-0"
              >
                Subagent: {{ session.agent_name || 'Worker' }}
              </span>

              <!-- Model Badge -->
              <span class="text-xs px-2.5 py-0.5 rounded font-mono font-semibold badge-cool flex-shrink-0">
                {{ session.model_name || 'Gemini' }}
              </span>

              <!-- Project Badge -->
              <span class="text-xs px-2.5 py-0.5 rounded font-mono font-medium badge-hit truncate max-w-xs flex-shrink-0">
                {{ session.project_name || 'Khác' }}
              </span>

              <!-- ID + Copy Button -->
              <div class="flex items-center gap-1 bg-paper px-2 py-0.5 rounded border border-rule text-xs font-mono text-ink-dim flex-shrink-0">
                <span>{{ session.id.slice(0, 8) }}</span>
                <button
                  @click="copySessionId"
                  class="text-cool hover:text-cyan-300 p-0.5 rounded transition cursor-pointer"
                  :title="copied ? 'Đã sao chép!' : 'Sao chép Full ID'"
                >
                  <svg v-if="!copied" class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                  </svg>
                  <svg v-else class="w-3 h-3 text-hit" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                </button>
              </div>

              <!-- Parent Link (if Subagent) -->
              <router-link
                v-if="session.parent_conversation_id"
                :to="{ name: 'session-detail', params: { id: session.parent_conversation_id } }"
                class="text-xs px-2 py-0.5 rounded bg-paper hover:bg-paper-subtle border border-purple-500/30 text-purple-300 transition hover:border-purple-400 flex items-center gap-1 font-mono"
                title="Bấm để xem phiên làm việc gốc (Parent)"
              >
                <span>↳ Thuộc Hội thoại cha:</span>
                <span class="font-bold underline">{{ session.parent_conversation_id.slice(0, 8) }}</span>
              </router-link>
            </div>

            <!-- Quick Metrics Summary & Toggle -->
            <div class="flex items-center gap-3 sm:gap-4 font-mono text-xs flex-shrink-0 ml-auto">
              <span class="text-ink-muted hidden md:inline">
                Steps: <strong class="text-cool">{{ session.step_count }}</strong>
              </span>
              <span class="text-ink-muted">
                Tokens: <strong class="text-ink">{{ formatNum(session.total_tokens) }}</strong>
              </span>
              <span
                class="px-2 py-0.5 rounded text-[11px] font-bold"
                :class="session.cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'"
              >
                Hit: {{ (session.cache_hit_rate || 0).toFixed(1) }}%
              </span>
              <span class="text-hit font-bold">
                {{ formatCurrencyUSD(session.cost_usd) }}
              </span>

              <!-- Toggle full KPI view -->
              <button
                @click="showFullKPIs = !showFullKPIs"
                class="p-1 rounded hover:bg-paper-subtle text-ink-muted hover:text-ink transition cursor-pointer"
                :title="showFullKPIs ? 'Thu gọn thống kê chi tiết' : 'Mở rộng thống kê chi tiết'"
              >
                <span class="text-[11px] font-mono underline mr-1 hidden sm:inline">
                  {{ showFullKPIs ? 'Thu gọn' : 'Chỉ số' }}
                </span>
                <span class="text-[10px]">{{ showFullKPIs ? '▲' : '▼' }}</span>
              </button>
            </div>
          </div>

          <!-- Collapsible Extended KPIs -->
          <div v-if="showFullKPIs" class="mt-4 pt-3 border-t border-rule/60 space-y-4">
            <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
              <div class="bg-paper border border-rule rounded-lg p-2.5 space-y-0.5 font-mono">
                <span class="text-[11px] text-ink-muted">Tổng Tokens</span>
                <div class="text-base font-bold text-ink">{{ formatNum(session.total_tokens) }}</div>
              </div>
              <div class="bg-paper border border-rule rounded-lg p-2.5 space-y-0.5 font-mono">
                <span class="text-[11px] text-ink-muted">Cache Hit Rate</span>
                <div class="text-base font-bold text-hit">{{ (session.cache_hit_rate || 0).toFixed(1) }}%</div>
              </div>
              <div class="bg-paper border border-rule rounded-lg p-2.5 space-y-0.5 font-mono">
                <span class="text-[11px] text-ink-muted">Cache Read</span>
                <div class="text-base font-bold text-hit">{{ formatNum(session.cache_read_tokens) }}</div>
              </div>
              <div class="bg-paper border border-rule rounded-lg p-2.5 space-y-0.5 font-mono">
                <span class="text-[11px] text-ink-muted">Input Mới</span>
                <div class="text-base font-bold text-accent">{{ formatNum(session.input_tokens) }}</div>
              </div>
              <div class="bg-paper border border-rule rounded-lg p-2.5 space-y-0.5 font-mono">
                <span class="text-[11px] text-ink-muted">Output Tokens</span>
                <div class="text-base font-bold text-output">{{ formatNum(session.output_tokens) }}</div>
              </div>
              <div class="bg-paper border border-rule rounded-lg p-2.5 space-y-0.5 font-mono">
                <span class="text-[11px] text-ink-muted">Chi phí VND</span>
                <div class="text-base font-bold text-hit">{{ formatCurrencyVND(session.cost_vnd) }}</div>
              </div>
            </div>

            <!-- Multiple Models Breakdown -->
            <div v-if="session.models && session.models.length > 1" class="flex flex-wrap gap-2 pt-1 font-mono text-xs">
              <span class="text-ink-muted self-center">Models:</span>
              <span
                v-for="m in session.models"
                :key="m.model_name"
                class="px-2.5 py-1 rounded bg-paper border border-rule text-ink flex items-center gap-2"
              >
                <strong>{{ m.model_name }}</strong>
                <span class="text-ink-dim">{{ m.step_count }} bước</span>
                <span class="text-hit font-bold">{{ formatCurrencyUSD(m.cost_usd) }}</span>
              </span>
            </div>
          </div>
        </div>

        <!-- Chat Viewport Container -->
        <div class="flex-1 min-h-0 bg-paper-card border border-rule rounded-2xl shadow-sm flex flex-col relative overflow-hidden">
          <!-- Chat Filter Toolbar -->
          <div class="px-4 py-2.5 bg-paper/70 border-b border-rule flex flex-wrap items-center justify-between gap-3 flex-shrink-0 z-10">
            <div class="flex items-center gap-2">
              <span class="text-sm">💬</span>
              <span class="font-bold text-xs sm:text-sm text-ink font-mono">Lịch sử Hội thoại</span>
              <span class="text-xs badge-cool px-2 py-0.5 rounded-full font-mono font-medium">
                {{ visibleSteps.length }} / {{ allFilteredSteps.length }} bước
              </span>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <!-- Search within chat -->
              <div class="relative min-w-[140px] sm:min-w-[180px]">
                <input
                  v-model="stepSearchQuery"
                  type="text"
                  placeholder="Tìm nội dung..."
                  class="w-full pl-6 pr-5 py-1 rounded-lg bg-paper border border-rule text-xs text-ink placeholder-ink-dim font-mono focus:outline-none focus:border-cool transition"
                />
                <span class="absolute left-2 top-1/2 -translate-y-1/2 text-ink-dim text-[10px]">🔍</span>
                <button
                  v-if="stepSearchQuery"
                  @click="stepSearchQuery = ''"
                  class="absolute right-1.5 top-1/2 -translate-y-1/2 text-ink-dim hover:text-ink text-xs"
                >
                  ✕
                </button>
              </div>

              <!-- Type Filters -->
              <div class="inline-flex rounded-lg bg-paper border border-rule p-0.5 text-xs font-mono">
                <button
                  @click="stepTypeFilter = 'all'"
                  class="px-2 py-0.5 rounded transition text-[11px]"
                  :class="stepTypeFilter === 'all' ? 'bg-cool text-paper font-bold' : 'text-ink-muted hover:text-ink'"
                >
                  Tất cả
                </button>
                <button
                  @click="stepTypeFilter = 'user'"
                  class="px-2 py-0.5 rounded transition text-[11px]"
                  :class="stepTypeFilter === 'user' ? 'bg-cool text-paper font-bold' : 'text-ink-muted hover:text-ink'"
                >
                  User
                </button>
                <button
                  @click="stepTypeFilter = 'model'"
                  class="px-2 py-0.5 rounded transition text-[11px]"
                  :class="stepTypeFilter === 'model' ? 'bg-cool text-paper font-bold' : 'text-ink-muted hover:text-ink'"
                >
                  AI
                </button>
                <button
                  @click="stepTypeFilter = 'tool'"
                  class="px-2 py-0.5 rounded transition text-[11px]"
                  :class="stepTypeFilter === 'tool' ? 'bg-cool text-paper font-bold' : 'text-ink-muted hover:text-ink'"
                >
                  Tool
                </button>
              </div>

              <!-- Expand/Collapse CoT -->
              <button
                @click="toggleAllThinking"
                class="px-2 py-1 rounded bg-paper hover:bg-paper-subtle border border-rule text-[11px] font-mono text-ink-muted hover:text-ink transition flex items-center gap-1 cursor-pointer"
                title="Mở rộng hoặc thu gọn toàn bộ CoT Thinking"
              >
                <span>🧠</span>
                <span class="hidden sm:inline">{{ allThinkingExpanded ? 'Thu gọn CoT' : 'Mở rộng CoT' }}</span>
              </button>

              <!-- Expand/Collapse Full Details -->
              <button
                @click="toggleAllDetails"
                class="px-2 py-1 rounded bg-paper hover:bg-paper-subtle border border-rule text-[11px] font-mono text-ink-muted hover:text-ink transition flex items-center gap-1 cursor-pointer"
                title="Mở rộng hoặc thu gọn toàn bộ nội dung chat chi tiết"
              >
                <span>📜</span>
                <span class="hidden sm:inline">{{ allDetailsExpanded ? 'Thu gọn chat' : 'Mở rộng full chat' }}</span>
              </button>
            </div>
          </div>

          <!-- Chat Scrollable Message Stream -->
          <div
            ref="chatContainerRef"
            @scroll="handleScroll"
            class="flex-1 min-h-0 overflow-y-auto p-4 sm:p-6 space-y-5 relative"
          >
            <!-- Top Indicator: Earlier Messages Prepend Trigger -->
            <div class="flex justify-center pb-2">
              <!-- If more earlier steps exist -->
              <button
                v-if="hasEarlierSteps"
                @click="loadEarlierSteps"
                :disabled="isLoadingEarlier"
                class="px-3.5 py-1.5 rounded-full bg-paper hover:bg-paper-subtle border border-rule text-xs font-mono text-cool hover:text-cyan-300 transition flex items-center gap-2 shadow-sm cursor-pointer group"
                title="Bấm hoặc cuộn lên trên để tải thêm tin nhắn cũ"
              >
                <svg
                  class="w-3.5 h-3.5"
                  :class="{ 'animate-spin': isLoadingEarlier, 'group-hover:-translate-y-0.5 transition-transform': !isLoadingEarlier }"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <line x1="12" y1="19" x2="12" y2="5"/>
                  <polyline points="5 12 12 5 19 12"/>
                </svg>
                <span>
                  {{ isLoadingEarlier ? 'Đang tải tin nhắn cũ...' : `Cuộn lên để tải thêm (${remainingEarlierCount} bước cũ)` }}
                </span>
              </button>

              <!-- When start of conversation is reached -->
              <div
                v-else-if="allFilteredSteps.length > 0"
                class="px-3 py-1 rounded-full bg-paper border border-rule-subtle text-[11px] font-mono text-ink-dim flex items-center gap-1.5"
              >
                <span>✦</span>
                <span>Bắt đầu cuộc hội thoại (Step 0)</span>
                <span>✦</span>
              </div>
            </div>

            <!-- Empty Search Results -->
            <div v-if="allFilteredSteps.length === 0" class="p-12 text-center text-ink-dim text-xs font-mono">
              Không tìm thấy tin nhắn nào phù hợp với bộ lọc tìm kiếm.
            </div>

            <!-- Messages Stream -->
            <template v-for="step in visibleSteps" :key="step.step_index">
              <!-- 1. USER MESSAGE BUBBLE -->
              <div
                v-if="step.source === 'USER_EXPLICIT' || step.type === 'USER_INPUT'"
                class="flex items-start gap-3 justify-end group pl-8 sm:pl-16"
              >
                <div class="flex flex-col items-end max-w-2xl sm:max-w-3xl w-full">
                  <!-- Meta Header -->
                  <div class="flex items-center gap-2 mb-1.5 text-[11px] font-mono text-ink-muted">
                    <span class="text-ink-dim">{{ formatTime(step.created_at) }}</span>
                    <span>•</span>
                    <span class="text-ink-dim">Step {{ step.step_index }}</span>
                    <span class="badge-cool px-2 py-0.5 rounded-full font-bold text-[10px]">
                      Bạn (User)
                    </span>
                  </div>

                  <!-- User Chat Bubble -->
                  <div class="bg-paper-card border border-cool-border rounded-2xl rounded-tr-sm p-4 text-xs font-mono text-ink whitespace-pre-wrap leading-relaxed shadow-sm selection:bg-cool/20 w-fit max-w-full">
                    <div v-if="!isStepExpanded(step.step_index)">
                      {{ cleanUserContent(step.content_preview) }}
                    </div>
                    <div v-else class="max-h-[600px] overflow-y-auto pr-1">
                      {{ cleanUserContent(step.content || step.content_preview) }}
                    </div>

                    <!-- Expand / Details Button -->
                    <button
                      v-if="hasDetailedContent(step)"
                      type="button"
                      @click="toggleStepExpanded(step.step_index)"
                      class="mt-2.5 px-2.5 py-1 rounded-md text-[11px] font-mono text-cool hover:text-paper bg-cool/10 hover:bg-cool transition flex items-center gap-1.5 cursor-pointer select-none border border-cool/20"
                      :title="isStepExpanded(step.step_index) ? 'Thu gọn lại' : 'Xem toàn bộ nội dung chi tiết'"
                    >
                      <span>{{ isStepExpanded(step.step_index) ? '▲ Thu gọn' : '🔍 Xem chi tiết đầy đủ' }}</span>
                      <span class="text-[10px] opacity-75 font-normal">({{ formatNum((step.content || '').length) }} ký tự)</span>
                    </button>
                  </div>
                </div>

                <!-- User Avatar -->
                <div class="w-8 h-8 rounded-full bg-cool/15 text-cool border border-cool/30 flex items-center justify-center font-bold text-xs flex-shrink-0 mt-1 shadow-sm select-none">
                  👤
                </div>
              </div>

              <!-- 2. SYSTEM MESSAGE PILL -->
              <div
                v-else-if="step.source === 'SYSTEM' || step.type === 'SYSTEM_MESSAGE'"
                class="flex justify-center my-1.5 px-4"
              >
                <div class="max-w-2xl px-3.5 py-1.5 rounded-xl bg-paper border border-rule text-[11px] font-mono text-ink-dim flex items-center gap-2 shadow-sm">
                  <span class="text-xs">⚙️</span>
                  <span class="truncate">{{ step.content_preview || 'System Message' }}</span>
                  <span class="text-[10px] text-ink-dim border-l border-rule pl-1.5 font-bold flex-shrink-0">
                    Step {{ step.step_index }}
                  </span>
                </div>
              </div>

              <!-- 3. ASSISTANT / MODEL RESPONSE BUBBLE -->
              <div
                v-else
                class="flex items-start gap-3 justify-start group pr-8 sm:pr-16"
              >
                <!-- Assistant Avatar -->
                <div class="w-8 h-8 rounded-full bg-paper border border-rule flex items-center justify-center flex-shrink-0 mt-1 shadow-sm select-none">
                  🤖
                </div>

                <div class="flex flex-col items-start max-w-3xl sm:max-w-4xl w-full">
                  <!-- Meta Header -->
                  <div class="flex flex-wrap items-center gap-2 mb-1.5 text-[11px] font-mono text-ink-muted">
                    <span class="font-bold text-ink">Antigravity AI</span>
                    <span v-if="step.model_name" class="badge-output px-2 py-0.5 rounded text-[10px] font-semibold">
                      {{ step.model_name }}
                    </span>
                    <span>• Step {{ step.step_index }}</span>
                    <span v-if="step.created_at" class="text-ink-dim">• {{ formatTime(step.created_at) }}</span>
                  </div>

                  <!-- Assistant Chat Bubble -->
                  <div class="bg-paper/90 border border-rule rounded-2xl rounded-tl-sm p-4 sm:p-5 text-xs text-ink space-y-3.5 shadow-sm w-full">
                    <!-- Thinking CoT Box (Expandable) -->
                    <div
                      v-if="step.thinking_preview"
                      class="rounded-xl border border-rule-subtle bg-paper-subtle/70 overflow-hidden text-[11px] font-mono"
                    >
                      <button
                        @click="toggleThinking(step.step_index)"
                        class="w-full px-3 py-2 flex items-center justify-between text-ink-muted hover:text-cool hover:bg-paper-subtle transition select-none cursor-pointer"
                      >
                        <span class="flex items-center gap-1.5 font-medium">
                          <span>🧠</span>
                          <span>{{ expandedThinking[step.step_index] ? 'Thu gọn suy nghĩ' : 'Xem quá trình suy nghĩ (Thinking CoT)' }}</span>
                        </span>
                        <span class="text-[10px]">{{ expandedThinking[step.step_index] ? '▲' : '▼' }}</span>
                      </button>
                      <div
                        v-if="expandedThinking[step.step_index]"
                        class="p-3.5 border-t border-rule/50 text-ink-dim whitespace-pre-wrap leading-relaxed max-h-96 overflow-y-auto selection:bg-cool/20"
                      >
                        {{ step.thinking || step.thinking_preview }}
                      </div>
                    </div>

                    <!-- Tool Execution Calls (if any) -->
                    <div v-if="step.tool_calls && step.tool_calls.length > 0" class="space-y-1.5">
                      <div class="text-[10px] font-mono uppercase tracking-wider text-ink-dim flex items-center gap-1">
                        <span>⚡ Công cụ thực thi</span>
                      </div>
                      <div class="flex flex-wrap gap-2">
                        <span
                          v-for="(t, tIdx) in step.tool_calls"
                          :key="tIdx"
                          class="px-2.5 py-1 rounded-lg text-xs font-mono badge-cool flex items-center gap-2 shadow-sm border border-cool-border/50"
                        >
                          <span>🔧</span>
                          <strong>{{ t.name }}</strong>
                          <span v-if="t.toolSummary" class="text-ink-muted text-[11px]">
                            ({{ t.toolSummary }})
                          </span>
                        </span>
                      </div>
                    </div>

                    <!-- Assistant Content -->
                    <div v-if="step.content_preview || step.content" class="space-y-2">
                      <div
                        v-if="!isStepExpanded(step.step_index)"
                        class="font-mono text-ink whitespace-pre-wrap leading-relaxed selection:bg-hit/20"
                      >
                        {{ step.content_preview }}
                      </div>
                      <div
                        v-else
                        class="font-mono text-ink whitespace-pre-wrap leading-relaxed selection:bg-hit/20 max-h-[650px] overflow-y-auto p-3 rounded-xl bg-paper-subtle/50 border border-rule/60 pr-2"
                      >
                        {{ step.content || step.content_preview }}
                      </div>

                      <!-- Expand / Details Button -->
                      <button
                        v-if="hasDetailedContent(step)"
                        type="button"
                        @click="toggleStepExpanded(step.step_index)"
                        class="mt-2 px-2.5 py-1 rounded-md text-[11px] font-mono text-cool hover:text-paper bg-cool/10 hover:bg-cool transition flex items-center gap-1.5 cursor-pointer select-none border border-cool/20 shadow-xs"
                        :title="isStepExpanded(step.step_index) ? 'Thu gọn lại' : 'Xem toàn bộ nội dung chi tiết'"
                      >
                        <span>{{ isStepExpanded(step.step_index) ? '▲ Thu gọn chi tiết' : '🔍 Xem chi tiết đầy đủ' }}</span>
                        <span class="text-[10px] opacity-75 font-normal">({{ formatNum((step.content || '').length) }} ký tự)</span>
                      </button>
                    </div>

                    <!-- Token Metrics Footer -->
                    <div
                      v-if="step.type === 'PLANNER_RESPONSE'"
                      class="pt-2 border-t border-rule/40 flex flex-wrap items-center justify-between gap-2 text-[11px] font-mono text-ink-dim"
                    >
                      <div class="flex items-center gap-3 flex-wrap">
                        <span title="Input Tokens">In: <strong class="text-accent">{{ formatNum(step.input_tokens) }}</strong></span>
                        <span title="Cache Read Tokens">
                          Cache Read: <strong class="text-hit">{{ formatNum(step.cache_read_tokens) }}</strong> ({{ (step.cache_hit_rate || 0).toFixed(1) }}%)
                        </span>
                        <span title="Output Tokens">Out: <strong class="text-output">{{ formatNum(step.output_tokens) }}</strong></span>
                      </div>
                      <div class="flex items-center gap-2">
                        <span class="px-2 py-0.5 rounded bg-paper border border-rule text-ink font-bold">
                          Total: {{ formatNum(step.total_step_tokens) }}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </template>
          </div>

          <!-- Floating "Scroll to Bottom" Button -->
          <button
            v-if="showScrollBottomBtn"
            @click="scrollToBottom(true)"
            class="absolute bottom-5 right-5 px-3 py-2 rounded-full bg-paper border border-cool text-cool hover:bg-cool hover:text-paper shadow-xl text-xs font-mono font-bold transition-all flex items-center gap-1.5 cursor-pointer z-20 animate-bounce"
            title="Cuộn xuống tin nhắn mới nhất"
          >
            <span>↓ Xuống tin mới</span>
          </button>
        </div>
      </template>
    </main>
  </div>
</template>
