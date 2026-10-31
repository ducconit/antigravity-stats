<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue';
import { useRouter } from 'vue-router';
import { serverState, fetchStats, fetchSessionDetail, disconnect } from '../stores/server';
import type { SessionMetric, ProjectStats } from '../types';
import MascotByte from '../components/MascotByte.vue';
import DashboardSkeleton from '../components/DashboardSkeleton.vue';
import WaterfallModal from '../components/WaterfallModal.vue';
import SearchableSelect, { type SelectOption } from '../components/SearchableSelect.vue';
import { Chart, registerables } from 'chart.js';

Chart.register(...registerables);

const router = useRouter();

// Search and filter state
const searchQuery = ref('');
const selectedProjectFilter = ref('');
const selectedModelFilter = ref('');
const selectedTypeFilter = ref('root');
const selectedSortBy = ref('recent');
const viewMode = ref<'project' | 'flat'>('project');
const collapsedProjects = ref<Record<string, boolean>>({});

// Modal state
const selectedSession = ref<SessionMetric | null>(null);
const isModalOpen = ref(false);

// Chart references
const modelChartCanvas = ref<HTMLCanvasElement | null>(null);
const timelineChartCanvas = ref<HTMLCanvasElement | null>(null);
let modelChartInstance: Chart | null = null;
let timelineChartInstance: Chart | null = null;

const stats = computed(() => serverState.stats);

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

function toggleProject(name: string) {
  collapsedProjects.value[name] = !collapsedProjects.value[name];
}

async function openSessionDetail(id: string) {
  const detail = await fetchSessionDetail(id);
  if (detail) {
    selectedSession.value = detail;
    isModalOpen.value = true;
  } else if (!serverState.isConnected) {
    router.push({ name: 'login' });
  }
}

async function handleRefresh() {
  const ok = await fetchStats(true);
  if (ok) {
    await nextTick();
    renderCharts();
  } else if (!serverState.isConnected) {
    router.push({ name: 'login' });
  }
}

function closeDetailModal() {
  isModalOpen.value = false;
  selectedSession.value = null;
}

function handleChangeServer() {
  disconnect();
  router.push({ name: 'login' });
}

// Options for Searchable Select Filters
const projectSelectOptions = computed<SelectOption[]>(() => {
  if (!stats.value?.projects) return [];
  return stats.value.projects.map((p) => ({
    value: p.name,
    label: p.name,
    count: p.session_count,
    subtitle: p.workspace_path || undefined,
  }));
});

const modelSelectOptions = computed<SelectOption[]>(() => {
  if (!stats.value) return [];
  const counts: Record<string, number> = {};
  for (const p of stats.value.projects || []) {
    for (const s of p.sessions || []) {
      const m = s.model_name || 'Khác';
      counts[m] = (counts[m] || 0) + 1;
    }
  }
  for (const m of Object.keys(stats.value.model_distribution || {})) {
    if (!counts[m]) counts[m] = 0;
  }
  return Object.keys(counts)
    .sort((a, b) => counts[b] - counts[a])
    .map((m) => ({
      value: m,
      label: m,
      count: counts[m],
    }));
});

const typeSelectOptions = computed<SelectOption[]>(() => {
  if (!stats.value) {
    return [
      { value: 'root', label: 'Hội thoại chính (Root)' },
      { value: 'all', label: 'Tất cả' },
      { value: 'subagent', label: 'Chỉ Subagent' },
    ];
  }
  let rootCount = 0;
  let subCount = 0;
  for (const p of stats.value.projects || []) {
    for (const s of p.sessions || []) {
      if (s.is_subagent) {
        subCount++;
      } else {
        rootCount++;
      }
    }
  }
  return [
    { value: 'root', label: 'Hội thoại chính (Root)', count: rootCount },
    { value: 'all', label: 'Tất cả hội thoại', count: rootCount + subCount },
    { value: 'subagent', label: 'Chỉ Subagent', count: subCount },
  ];
});

const sortSelectOptions: SelectOption[] = [
  { value: 'recent', label: 'Mới nhất' },
  { value: 'oldest', label: 'Cũ nhất' },
  { value: 'tokens-desc', label: 'Nhiều Token nhất' },
  { value: 'tokens-asc', label: 'Ít Token nhất' },
  { value: 'cost-desc', label: 'Chi phí cao nhất' },
  { value: 'cache-desc', label: 'Cache Hit cao nhất' },
];

const isAnyFilterActive = computed(() => {
  return Boolean(
    searchQuery.value.trim() ||
      selectedProjectFilter.value ||
      selectedModelFilter.value ||
      selectedTypeFilter.value !== 'root' ||
      selectedSortBy.value !== 'recent'
  );
});

function resetAllFilters() {
  searchQuery.value = '';
  selectedProjectFilter.value = '';
  selectedModelFilter.value = '';
  selectedTypeFilter.value = 'root';
  selectedSortBy.value = 'recent';
}

function compareSessions(a: SessionMetric, b: SessionMetric, sort: string): number {
  if (sort === 'oldest') {
    return new Date(a.last_modified).getTime() - new Date(b.last_modified).getTime();
  }
  if (sort === 'tokens-desc') {
    return (b.total_tokens || 0) - (a.total_tokens || 0);
  }
  if (sort === 'tokens-asc') {
    return (a.total_tokens || 0) - (b.total_tokens || 0);
  }
  if (sort === 'cost-desc') {
    return (b.cost_usd || 0) - (a.cost_usd || 0);
  }
  if (sort === 'cache-desc') {
    return (b.cache_hit_rate || 0) - (a.cache_hit_rate || 0);
  }
  // Default 'recent'
  return new Date(b.last_modified).getTime() - new Date(a.last_modified).getTime();
}

// Filtered Projects & Sessions
const filteredProjects = computed<ProjectStats[]>(() => {
  if (!stats.value?.projects) return [];
  const q = searchQuery.value.trim().toLowerCase();
  const pFilter = selectedProjectFilter.value;
  const mFilter = selectedModelFilter.value;
  const tFilter = selectedTypeFilter.value;
  const sort = selectedSortBy.value;

  let list = stats.value.projects;
  if (pFilter) {
    list = list.filter((p) => p.name === pFilter);
  }

  const mapped = list
    .map((p) => {
      const matchedSessions = p.sessions.filter((s) => {
        if (tFilter === 'root' && s.is_subagent) return false;
        if (tFilter === 'subagent' && !s.is_subagent) return false;

        const matchQ =
          !q ||
          p.name.toLowerCase().includes(q) ||
          s.id.toLowerCase().includes(q) ||
          s.title.toLowerCase().includes(q) ||
          (p.workspace_path && p.workspace_path.toLowerCase().includes(q));
        const matchM = !mFilter || s.model_name === mFilter || s.models?.some((m) => m.model_name === mFilter);
        return matchQ && matchM;
      });

      matchedSessions.sort((a, b) => compareSessions(a, b, sort));

      return {
        ...p,
        sessions: matchedSessions,
      };
    })
    .filter((p) => p.sessions.length > 0);

  mapped.sort((a, b) => {
    if (sort === 'tokens-desc') return (b.total_tokens || 0) - (a.total_tokens || 0);
    if (sort === 'tokens-asc') return (a.total_tokens || 0) - (b.total_tokens || 0);
    if (sort === 'cost-desc') return (b.total_cost_usd || 0) - (a.total_cost_usd || 0);
    if (sort === 'cache-desc') return (b.overall_cache_hit_rate || 0) - (a.overall_cache_hit_rate || 0);
    if (sort === 'oldest') {
      const timeA = a.sessions[0] ? new Date(a.sessions[0].last_modified).getTime() : 0;
      const timeB = b.sessions[0] ? new Date(b.sessions[0].last_modified).getTime() : 0;
      return timeA - timeB;
    }
    // 'recent'
    const timeA = a.sessions[0] ? new Date(a.sessions[0].last_modified).getTime() : 0;
    const timeB = b.sessions[0] ? new Date(b.sessions[0].last_modified).getTime() : 0;
    return timeB - timeA;
  });

  return mapped;
});

const allFlatSessions = computed<SessionMetric[]>(() => {
  if (!filteredProjects.value) return [];
  const list: SessionMetric[] = [];
  for (const p of filteredProjects.value) {
    list.push(...p.sessions);
  }
  return list.sort((a, b) => compareSessions(a, b, selectedSortBy.value));
});

function renderCharts() {
  if (!stats.value) return;

  // 1. Donut Chart for Models
  if (modelChartCanvas.value) {
    if (modelChartInstance) modelChartInstance.destroy();
    const rawLabels = Object.keys(stats.value.model_distribution || {});
    const modelLabels = rawLabels.map((lbl) => {
      const clean = lbl.replace(/\n.*/s, '').trim();
      return clean.length > 22 ? clean.slice(0, 20) + '...' : clean;
    });
    const modelData = Object.values(stats.value.model_distribution || {});
    const palette = ['#38bdf8', '#10b981', '#f59e0b', '#a855f7', '#ec4899', '#6366f1'];

    modelChartInstance = new Chart(modelChartCanvas.value, {
      type: 'doughnut',
      data: {
        labels: modelLabels,
        datasets: [
          {
            data: modelData,
            backgroundColor: palette.slice(0, modelLabels.length),
            borderWidth: 2,
            borderColor: '#161b22',
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'bottom',
            labels: { color: '#8b949e', font: { family: 'JetBrains Mono', size: 11 } },
          },
        },
      },
    });
  }

  // 2. Daily Timeline Bar Chart
  if (timelineChartCanvas.value && stats.value.daily_metrics) {
    if (timelineChartInstance) timelineChartInstance.destroy();
    const sortedDays = [...stats.value.daily_metrics].sort((a, b) => a.date.localeCompare(b.date));
    const labels = sortedDays.map((d) => d.date);
    const cacheData = sortedDays.map((d) => d.cache_read_tokens);
    const inputData = sortedDays.map((d) => d.input_tokens);

    timelineChartInstance = new Chart(timelineChartCanvas.value, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: 'Cache Read Tokens',
            data: cacheData,
            backgroundColor: 'rgba(16, 185, 129, 0.7)',
            borderColor: '#10b981',
            borderWidth: 1,
            borderRadius: 4,
          },
          {
            label: 'Input Tokens Mới',
            data: inputData,
            backgroundColor: 'rgba(245, 158, 11, 0.7)',
            borderColor: '#f59e0b',
            borderWidth: 1,
            borderRadius: 4,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: {
            stacked: true,
            ticks: { color: '#8b949e', font: { family: 'JetBrains Mono', size: 10 } },
            grid: { display: false },
          },
          y: {
            stacked: true,
            ticks: { color: '#8b949e', font: { family: 'JetBrains Mono', size: 10 } },
            grid: { color: '#21262d' },
          },
        },
        plugins: {
          legend: {
            position: 'top',
            labels: { color: '#8b949e', font: { family: 'JetBrains Mono', size: 11 } },
          },
        },
      },
    });
  }
}

onMounted(async () => {
  const ok = await fetchStats();
  if (ok) {
    await nextTick();
    renderCharts();
  } else {
    // If failed to fetch stats, go back to login
    router.push({ name: 'login' });
  }
});
</script>

<template>
  <div class="min-h-screen bg-paper pb-16">
    <!-- Header -->
    <header class="border-b border-rule bg-paper-card/70 backdrop-blur sticky top-0 z-30">
      <div class="w-full px-4 sm:px-6 lg:px-8 xl:px-10 2xl:px-12 py-3.5 flex items-center justify-between gap-4">
        <!-- Logo & Title -->
        <div class="flex items-center gap-3 min-w-0">
          <div class="p-2 rounded-xl bg-paper border border-rule flex-shrink-0">
            <svg class="w-5 h-5 text-cool" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="4" y="6" width="16" height="12" rx="3"/>
              <line x1="12" y1="2" x2="12" y2="6"/>
              <circle cx="9" cy="11" r="1.5" fill="#10b981"/>
              <circle cx="15" cy="11" r="1.5" fill="#10b981"/>
              <path d="M9 15h6"/>
            </svg>
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h1 class="font-bold text-sm sm:text-base text-ink truncate">
                Antigravity Token Visualizer
              </h1>
              <span class="text-xs badge-accent px-2 py-0.5 rounded-full font-mono font-semibold">
                v1.0
              </span>
            </div>
            <p class="text-xs text-ink-muted hidden sm:block truncate">
              LLM Token Consumption, Cache Hit &amp; Cost Analytics
            </p>
          </div>
        </div>

        <!-- Server Status & Actions -->
        <div class="flex items-center gap-2.5 flex-shrink-0">
          <!-- Server Status Pill -->
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

          <!-- Refresh Button -->
          <button
            @click="handleRefresh"
            :disabled="serverState.loading"
            class="px-2.5 sm:px-3 py-1.5 rounded-lg bg-paper-card hover:bg-paper-subtle border border-rule text-xs font-medium text-ink transition flex items-center gap-1.5 shadow"
          >
            <svg
              class="w-3.5 h-3.5 text-cool"
              :class="{ 'animate-spin': serverState.loading }"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
            </svg>
            <span class="hidden sm:inline">Làm mới</span>
          </button>

          <!-- Author link -->
          <a
            href="https://github.com/ducconit"
            target="_blank"
            rel="noopener noreferrer"
            class="px-2.5 sm:px-3 py-1.5 rounded-lg bg-paper-card hover:bg-paper-subtle border border-rule text-xs font-medium text-ink hover:text-cool transition flex items-center gap-1.5 shadow hidden md:flex"
          >
            <span>Tác giả: <strong class="text-cool">DNT</strong></span>
          </a>
        </div>
      </div>
    </header>

    <!-- Interactive Mascot Bar -->
    <MascotByte />

    <main class="w-full px-4 sm:px-6 lg:px-8 xl:px-10 2xl:px-12 mt-6 space-y-6">
      <!-- Loading Skeleton while connecting or fetching stats -->
      <DashboardSkeleton v-if="!stats" />

      <!-- Loaded Dashboard Content -->
      <template v-else>
        <!-- KPI Overview Cards -->
        <div class="grid grid-cols-2 lg:grid-cols-5 gap-3.5 sm:gap-4">
        <!-- 1. Cache Hit Tokens -->
        <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow">
          <div class="text-xs text-ink-muted font-medium mb-1">Cache Hit Tokens</div>
          <div class="text-xl sm:text-2xl font-bold font-mono text-hit leading-tight">
            {{ formatNum(stats.total_cache_read) }}
          </div>
          <p class="text-[11px] text-ink-dim mt-2 truncate">
            Đã tái sử dụng từ bộ nhớ đệm
          </p>
        </div>

        <!-- 2. Cache Hit Rate -->
        <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow">
          <div class="text-xs text-ink-muted font-medium mb-1">Tỷ lệ Cache Hit</div>
          <div class="text-xl sm:text-2xl font-bold font-mono text-cool leading-tight">
            {{ (stats.overall_cache_hit_rate || 0).toFixed(1) }}%
          </div>
          <p class="text-[11px] text-ink-dim mt-2 truncate">
            Tiết kiệm ~75-90% chi phí
          </p>
        </div>

        <!-- 3. New Input Tokens -->
        <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow">
          <div class="text-xs text-ink-muted font-medium mb-1">Input Tokens Mới</div>
          <div class="text-xl sm:text-2xl font-bold font-mono text-accent leading-tight">
            {{ formatNum(stats.total_input_tokens) }}
          </div>
          <p class="text-[11px] text-ink-dim mt-2 truncate">
            Prompt &amp; Tool outputs
          </p>
        </div>

        <!-- 4. Output Tokens -->
        <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow">
          <div class="text-xs text-ink-muted font-medium mb-1">Output Tokens</div>
          <div class="text-xl sm:text-2xl font-bold font-mono text-output leading-tight">
            {{ formatNum(stats.total_output) }}
          </div>
          <p class="text-[11px] text-ink-dim mt-2 truncate">
            Thinking + Responses
          </p>
        </div>

        <!-- 5. Total Cost -->
        <div class="col-span-2 lg:col-span-1 bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow">
          <div class="text-xs text-ink-muted font-medium mb-1">Chi phí ước tính</div>
          <div class="text-xl sm:text-2xl font-bold font-mono text-hit leading-tight">
            {{ formatCurrencyUSD(stats.total_cost_usd) }}
          </div>
          <p class="text-[11px] text-ink-dim mt-2 truncate">
            {{ formatCurrencyVND(stats.total_cost_vnd) }}
          </p>
        </div>
      </div>

      <!-- Charts Section -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- Model Distribution Chart -->
        <div class="bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col">
          <h3 class="text-sm font-bold text-ink mb-1">Phân bổ Token theo Model</h3>
          <p class="text-xs text-ink-dim mb-4">Tỷ lệ sử dụng các họ mô hình AI</p>
          <div class="relative h-64 w-full flex-1">
            <canvas ref="modelChartCanvas"></canvas>
          </div>
        </div>

        <!-- Daily Consumption Timeline -->
        <div class="lg:col-span-2 bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col">
          <h3 class="text-sm font-bold text-ink mb-1">Dòng thời gian tiêu thụ Token (Daily)</h3>
          <p class="text-xs text-ink-dim mb-4">So sánh Token mới vs Cache Read qua các ngày</p>
          <div class="relative h-64 w-full flex-1">
            <canvas ref="timelineChartCanvas"></canvas>
          </div>
        </div>
      </div>

      <!-- Sessions Explorer Section -->
      <div class="bg-paper-card border border-rule rounded-2xl shadow overflow-hidden">
        <!-- Explorer Header: Row 1 (Title, Counts & View Mode Switch) -->
        <div class="p-4 sm:p-5 border-b border-rule/60 flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div class="min-w-0">
            <div class="flex items-center gap-2.5 flex-wrap">
              <h2 class="text-sm sm:text-base font-bold text-ink">Khám phá Phiên làm việc (Sessions Explorer)</h2>
              <span class="text-xs badge-cool px-2.5 py-0.5 rounded-full font-mono font-medium">
                {{ viewMode === 'project' ? `${filteredProjects.length} / ${stats?.projects?.length || 0} dự án` : `${allFlatSessions.length} hội thoại` }}
              </span>
              <span v-if="isAnyFilterActive" class="text-[11px] badge-accent px-2 py-0.5 rounded-full font-mono font-medium">
                Đang lọc
              </span>
            </div>
            <p class="text-xs text-ink-dim mt-1">
              Phân cấp theo Dự án &rarr; Hội thoại &rarr; Nhóm Model &rarr; Waterfall Steps
            </p>
          </div>

          <!-- Controls: View Toggle -->
          <div class="flex items-center gap-3 shrink-0 self-start md:self-center">
            <div class="flex rounded-lg border border-rule overflow-hidden bg-paper shadow-sm">
              <button
                @click="viewMode = 'project'"
                class="px-3 py-1.5 text-xs font-mono transition flex items-center gap-1.5 cursor-pointer"
                :class="viewMode === 'project' ? 'bg-cool text-paper font-semibold shadow-sm' : 'text-ink-muted hover:text-ink'"
              >
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                </svg>
                <span>Dự án</span>
              </button>
              <button
                @click="viewMode = 'flat'"
                class="px-3 py-1.5 text-xs font-mono transition flex items-center gap-1.5 cursor-pointer"
                :class="viewMode === 'flat' ? 'bg-cool text-paper font-semibold shadow-sm' : 'text-ink-muted hover:text-ink'"
              >
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="8" y1="6" x2="21" y2="6"/>
                  <line x1="8" y1="12" x2="21" y2="12"/>
                  <line x1="8" y1="18" x2="21" y2="18"/>
                  <line x1="3" y1="6" x2="3.01" y2="6"/>
                  <line x1="3" y1="12" x2="3.01" y2="12"/>
                  <line x1="3" y1="18" x2="3.01" y2="18"/>
                </svg>
                <span>Phẳng</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Explorer Filter Toolbar: Row 2 (Search + Searchable Select Filters) -->
        <div class="p-3 sm:px-5 sm:py-3.5 bg-paper/40 border-b border-rule flex flex-wrap items-center gap-2.5 sm:gap-3">
          <!-- Search Input -->
          <div class="relative flex-1 min-w-[180px] sm:min-w-[240px] max-w-sm">
            <svg
              class="w-3.5 h-3.5 text-ink-dim absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <circle cx="11" cy="11" r="8"/>
              <line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="Tìm dự án, hội thoại, ID..."
              class="w-full pl-8 pr-7 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink placeholder-ink-dim font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool transition"
            />
            <button
              v-if="searchQuery"
              type="button"
              @click="searchQuery = ''"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-ink-dim hover:text-ink p-0.5 cursor-pointer"
              title="Xóa tìm kiếm"
            >
              <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>

          <!-- Project Searchable Select -->
          <SearchableSelect
            v-model="selectedProjectFilter"
            :options="projectSelectOptions"
            placeholder="Tất cả Dự án"
            search-placeholder="Tìm kiếm dự án..."
            icon="folder"
            clearable
          />

          <!-- Model Searchable Select -->
          <SearchableSelect
            v-model="selectedModelFilter"
            :options="modelSelectOptions"
            placeholder="Tất cả Model"
            search-placeholder="Tìm kiếm model..."
            icon="sparkles"
            clearable
          />

          <!-- Type Searchable Select -->
          <SearchableSelect
            v-model="selectedTypeFilter"
            :options="typeSelectOptions"
            placeholder="Loại hội thoại"
            search-placeholder="Tìm kiếm loại..."
            icon="filter"
            :show-all-option="false"
            :clearable="false"
          />

          <!-- Sort Searchable Select -->
          <SearchableSelect
            v-model="selectedSortBy"
            :options="sortSelectOptions"
            placeholder="Sắp xếp"
            search-placeholder="Tìm kiểu sắp xếp..."
            icon="sort"
            :show-all-option="false"
          />

          <!-- Reset All Filters Pill Button -->
          <button
            v-if="isAnyFilterActive"
            type="button"
            @click="resetAllFilters"
            class="text-xs font-mono text-accent hover:text-accent/80 flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-accent-soft/40 border border-accent-border transition cursor-pointer hover:bg-accent-soft ml-auto sm:ml-0"
            title="Xóa toàn bộ bộ lọc đang áp dụng"
          >
            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"/>
              <line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
            <span>Đặt lại bộ lọc</span>
          </button>
        </div>

        <!-- Project View Mode (Accordion) -->
        <div v-if="viewMode === 'project'" class="divide-y divide-rule">
          <div v-if="filteredProjects.length === 0" class="p-8 text-center text-ink-dim text-xs font-mono">
            Không tìm thấy dự án hoặc phiên làm việc phù hợp.
          </div>

          <div v-for="p in filteredProjects" :key="p.name" class="transition">
            <!-- Project Header Row -->
            <div
              @click="toggleProject(p.name)"
              class="p-4 hover:bg-paper-subtle/50 cursor-pointer flex items-center justify-between gap-4 transition select-none"
            >
              <div class="flex items-center gap-3 min-w-0">
                <span class="text-xs text-ink-dim font-mono transition-transform" :class="{ 'rotate-90': !collapsedProjects[p.name] }">
                  ▶
                </span>
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-sm text-ink truncate font-mono">{{ p.name }}</span>
                    <span class="text-[11px] badge-cool px-2 py-0.5 rounded-full font-mono">
                      {{ p.session_count }} hội thoại
                    </span>
                  </div>
                  <p v-if="p.workspace_path" class="text-[11px] text-ink-dim truncate font-mono mt-0.5">
                    {{ p.workspace_path }}
                  </p>
                </div>
              </div>

              <!-- Project Summary Stats -->
              <div class="flex items-center gap-3 sm:gap-4 font-mono text-xs flex-shrink-0">
                <span class="text-ink-muted hidden sm:inline">{{ formatNum(p.total_tokens) }} tokens</span>
                <span class="px-2 py-0.5 rounded text-[11px] font-bold" :class="p.overall_cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'">
                  Hit: {{ (p.overall_cache_hit_rate || 0).toFixed(1) }}%
                </span>
                <span class="text-hit font-bold hidden md:inline">
                  {{ formatCurrencyUSD(p.total_cost_usd) }}
                </span>
              </div>
            </div>

            <!-- Sessions List under Project -->
            <div v-if="!collapsedProjects[p.name]" class="bg-paper/40 divide-y divide-rule/60 border-t border-rule/60">
              <div
                v-for="s in p.sessions"
                :key="s.id"
                @click="openSessionDetail(s.id)"
                class="px-6 sm:px-8 py-3 hover:bg-paper-subtle/70 cursor-pointer flex items-center justify-between gap-4 transition text-xs group"
              >
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span
                      v-if="s.is_subagent"
                      class="text-[10px] px-1.5 py-0.5 rounded font-mono flex-shrink-0 bg-purple-500/15 text-purple-300 border border-purple-500/30"
                      :title="s.parent_conversation_id ? `Subagent của ${s.parent_conversation_id.slice(0, 8)}` : 'Subagent'"
                    >
                      Subagent<span v-if="s.agent_name">: {{ s.agent_name }}</span>
                    </span>
                    <span class="font-medium text-ink group-hover:text-cool transition truncate font-mono">
                      {{ s.title }}
                    </span>
                    <span class="text-[10px] badge-subtle px-1.5 py-0.5 rounded font-mono flex-shrink-0">
                      {{ s.id.substring(0, 8) }}
                    </span>
                  </div>
                  <div class="flex items-center gap-3 text-[11px] text-ink-dim mt-1 font-mono">
                    <span>{{ new Date(s.last_modified).toLocaleString('vi-VN') }}</span>
                    <span>• {{ s.step_count }} bước</span>
                    <span>• {{ s.model_name }}</span>
                  </div>
                </div>

                <div class="flex items-center gap-3 font-mono flex-shrink-0">
                  <span class="text-ink-muted hidden sm:inline">{{ formatNum(s.total_tokens) }}</span>
                  <span class="px-2 py-0.5 rounded text-[10px] font-bold" :class="s.cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'">
                    {{ s.cache_hit_rate.toFixed(1) }}%
                  </span>
                  <span class="text-hit font-bold">
                    {{ formatCurrencyUSD(s.cost_usd) }}
                  </span>
                  <span class="text-cool group-hover:translate-x-1 transition-transform">
                    &rarr;
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Flat Sessions View Mode -->
        <div v-else class="divide-y divide-rule">
          <div
            v-for="s in allFlatSessions"
            :key="s.id"
            @click="openSessionDetail(s.id)"
            class="p-4 hover:bg-paper-subtle/70 cursor-pointer flex items-center justify-between gap-4 transition text-xs group"
          >
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span
                  v-if="s.is_subagent"
                  class="text-[10px] px-1.5 py-0.5 rounded font-mono flex-shrink-0 bg-purple-500/15 text-purple-300 border border-purple-500/30"
                  :title="s.parent_conversation_id ? `Subagent của ${s.parent_conversation_id.slice(0, 8)}` : 'Subagent'"
                >
                  Subagent<span v-if="s.agent_name">: {{ s.agent_name }}</span>
                </span>
                <span class="font-semibold text-ink group-hover:text-cool transition truncate font-mono">
                  {{ s.title }}
                </span>
                <span class="text-[10px] badge-hit px-2 py-0.5 rounded font-medium truncate max-w-xs sm:max-w-sm lg:max-w-md 2xl:max-w-lg">
                  {{ s.project_name }}
                </span>
              </div>
              <div class="flex items-center gap-3 text-[11px] text-ink-dim mt-1 font-mono">
                <span>{{ new Date(s.last_modified).toLocaleString('vi-VN') }}</span>
                <span>• {{ s.step_count }} bước</span>
                <span>• {{ s.model_name }}</span>
              </div>
            </div>

            <div class="flex items-center gap-3 font-mono flex-shrink-0">
              <span class="text-ink-muted hidden sm:inline">{{ formatNum(s.total_tokens) }}</span>
              <span class="px-2 py-0.5 rounded text-[10px] font-bold" :class="s.cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'">
                {{ s.cache_hit_rate.toFixed(1) }}%
              </span>
              <span class="text-hit font-bold">
                {{ formatCurrencyUSD(s.cost_usd) }}
              </span>
              <span class="text-cool group-hover:translate-x-1 transition-transform">
                &rarr;
              </span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </main>

    <!-- Step Waterfall Modal -->
    <WaterfallModal
      :session="selectedSession"
      :is-open="isModalOpen"
      @close="closeDetailModal"
    />
  </div>
</template>
