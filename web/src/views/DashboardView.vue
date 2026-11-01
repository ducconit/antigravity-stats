<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue';
import { useRouter } from 'vue-router';
import { Chart, registerables } from 'chart.js';
import { serverState, fetchStats, disconnect } from '../stores/server';
import DashboardSkeleton from '../components/DashboardSkeleton.vue';
import SearchableSelect from '../components/SearchableSelect.vue';
import type { SelectOption } from '../components/SearchableSelect.vue';
import type { ProjectStats, SessionMetric, ModelUsageStat, AgentToolStat } from '../types';

Chart.register(...registerables);

const router = useRouter();

// Main Dashboard Active Tab
const activeDashboardTab = ref<'overview' | 'projects' | 'models' | 'tools' | 'sessions'>('overview');

// Sessions Explorer state
const searchQuery = ref('');
const selectedProjectFilter = ref<string>('');
const selectedModelFilter = ref<string>('');
const selectedToolFilter = ref<string>('');
const selectedTypeFilter = ref<string>('root'); // 'root' | 'all' | 'subagent'
const selectedSortBy = ref<string>('recent');
const viewMode = ref<'project' | 'flat'>('project');
const collapsedProjects = ref<Record<string, boolean>>({});

// Projects Tab state
const projectSearchQuery = ref('');
const projectSortBy = ref<'tokens' | 'cost' | 'cache' | 'sessions' | 'recent' | 'name'>('tokens');

// Models Tab state
const modelSearchQuery = ref('');
const modelSortBy = ref<'tokens' | 'cost' | 'cache' | 'steps' | 'name'>('tokens');

// Tools Tab state
const agentToolSearchQuery = ref('');
const selectedAgentToolCategory = ref<string>('all');

// Chart refs & instances
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

function openSessionDetail(id: string) {
  router.push({ name: 'session-detail', params: { id } });
}

async function handleRefresh() {
  const ok = await fetchStats(true);
  if (ok) {
    await nextTick();
    if (activeDashboardTab.value === 'overview') {
      renderCharts();
    }
  } else if (!serverState.isConnected) {
    router.push({ name: 'login' });
  }
}

function handleChangeServer() {
  disconnect();
  router.push({ name: 'login' });
}

// Navigation helpers between tabs
function navigateToProjectSessions(projectName: string) {
  selectedProjectFilter.value = projectName;
  activeDashboardTab.value = 'sessions';
}

function navigateToModelSessions(modelName: string) {
  selectedModelFilter.value = modelName;
  activeDashboardTab.value = 'sessions';
}

function navigateToToolSessions(toolName: string) {
  selectedToolFilter.value = toolName;
  activeDashboardTab.value = 'sessions';
}

// Options for Searchable Select Filters in Sessions Explorer
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

const toolSelectOptions = computed<SelectOption[]>(() => {
  if (!stats.value?.tools) {
    return [
      { value: 'Antigravity App', label: '🖥️ Antigravity App' },
      { value: 'Antigravity CLI', label: '⌨️ Antigravity CLI' },
      { value: 'Antigravity IDE', label: '💻 Antigravity IDE' },
    ];
  }
  return stats.value.tools.map((t) => ({
    value: t.name,
    label: `${t.icon === 'desktop' ? '🖥️' : t.icon === 'terminal' ? '⌨️' : '💻'} ${t.name}`,
    count: t.session_count,
    subtitle: `${formatNum(t.total_tokens)} tokens`,
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
      selectedToolFilter.value ||
      selectedTypeFilter.value !== 'root' ||
      selectedSortBy.value !== 'recent'
  );
});

function resetAllFilters() {
  searchQuery.value = '';
  selectedProjectFilter.value = '';
  selectedModelFilter.value = '';
  selectedToolFilter.value = '';
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

// Filtered Projects & Sessions for Sessions Explorer
const filteredProjects = computed<ProjectStats[]>(() => {
  if (!stats.value?.projects) return [];
  const q = searchQuery.value.trim().toLowerCase();
  const pFilter = selectedProjectFilter.value;
  const mFilter = selectedModelFilter.value;
  const tFilter = selectedToolFilter.value;
  const typeFilter = selectedTypeFilter.value;
  const sort = selectedSortBy.value;

  let list = stats.value.projects;
  if (pFilter) {
    list = list.filter((p) => p.name === pFilter);
  }

  const mapped = list
    .map((p) => {
      const matchedSessions = p.sessions.filter((s) => {
        if (typeFilter === 'root' && s.is_subagent) return false;
        if (typeFilter === 'subagent' && !s.is_subagent) return false;
        if (tFilter && s.client_tool !== tFilter) return false;

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

// Subagents hierarchy in Sessions Explorer
const expandedSubagentSessions = ref<Record<string, boolean>>({});

function toggleSubagents(sessionID: string) {
  expandedSubagentSessions.value[sessionID] = !expandedSubagentSessions.value[sessionID];
}

const subagentsByParentId = computed<Record<string, SessionMetric[]>>(() => {
  if (!stats.value?.projects) return {};
  const map: Record<string, SessionMetric[]> = {};
  for (const p of stats.value.projects) {
    for (const s of p.sessions) {
      if (s.is_subagent && s.parent_conversation_id) {
        if (!map[s.parent_conversation_id]) {
          map[s.parent_conversation_id] = [];
        }
        map[s.parent_conversation_id].push(s);
      }
    }
  }
  return map;
});

// Sorted Projects List for Dedicated Projects Tab
const sortedProjectsList = computed<ProjectStats[]>(() => {
  if (!stats.value?.projects) return [];
  const q = projectSearchQuery.value.trim().toLowerCase();
  let list = stats.value.projects.filter((p) => {
    return !q || p.name.toLowerCase().includes(q) || (p.workspace_path && p.workspace_path.toLowerCase().includes(q));
  });

  const s = projectSortBy.value;
  return [...list].sort((a, b) => {
    if (s === 'tokens') return (b.total_tokens || 0) - (a.total_tokens || 0);
    if (s === 'cost') return (b.total_cost_usd || 0) - (a.total_cost_usd || 0);
    if (s === 'cache') return (b.overall_cache_hit_rate || 0) - (a.overall_cache_hit_rate || 0);
    if (s === 'sessions') return (b.session_count || 0) - (a.session_count || 0);
    if (s === 'name') return a.name.localeCompare(b.name);
    // recent
    return new Date(b.last_modified).getTime() - new Date(a.last_modified).getTime();
  });
});

// Models List for Dedicated Models Tab
const sortedModelsList = computed<ModelUsageStat[]>(() => {
  if (!stats.value?.models) return [];
  const q = modelSearchQuery.value.trim().toLowerCase();
  let list = stats.value.models.filter((m) => !q || m.model_name.toLowerCase().includes(q));

  const s = modelSortBy.value;
  return [...list].sort((a, b) => {
    if (s === 'tokens') return (b.total_tokens || 0) - (a.total_tokens || 0);
    if (s === 'cost') return (b.cost_usd || 0) - (a.cost_usd || 0);
    if (s === 'cache') return (b.overall_cache_hit_rate || 0) - (a.overall_cache_hit_rate || 0);
    if (s === 'steps') return (b.step_count || 0) - (a.step_count || 0);
    return a.model_name.localeCompare(b.model_name);
  });
});

// Agent Tools for Tools Tab
const agentToolCategories = computed<string[]>(() => {
  if (!stats.value?.agent_tools) return ['all'];
  const cats = new Set<string>();
  for (const t of stats.value.agent_tools) {
    if (t.category) cats.add(t.category);
    if (t.server_group?.toLowerCase().includes('native')) {
      cats.add('Native');
    }
  }
  return ['all', ...Array.from(cats).sort()];
});

const totalAgentToolCalls = computed<number>(() => {
  if (!stats.value?.agent_tools) return 0;
  return stats.value.agent_tools.reduce((acc, t) => acc + (t.call_count || 0), 0);
});

const filteredAgentTools = computed<AgentToolStat[]>(() => {
  if (!stats.value?.agent_tools) return [];
  const q = agentToolSearchQuery.value.trim().toLowerCase();
  const cat = selectedAgentToolCategory.value;

  return stats.value.agent_tools.filter((t) => {
    const matchQ = !q || t.name.toLowerCase().includes(q) || (t.server_group && t.server_group.toLowerCase().includes(q));
    const matchCat =
      cat === 'all' ||
      t.category === cat ||
      (cat === 'Native' && t.server_group?.includes('Native'));
    return matchQ && matchCat;
  });
});

export interface ToolGroupEntry {
  isGroup: true;
  groupName: string;
  category: string;
  callCount: number;
  tools: AgentToolStat[];
}

export interface ToolSingleEntry {
  isGroup: false;
  tool: AgentToolStat;
}

export type DisplayToolEntry = ToolGroupEntry | ToolSingleEntry;

const toolViewMode = ref<'grouped' | 'flat'>('grouped');
const expandedToolGroups = ref<Record<string, boolean>>({});

function toggleToolGroup(groupName: string) {
  expandedToolGroups.value[groupName] = !expandedToolGroups.value[groupName];
}

function expandAllToolGroups() {
  for (const entry of displayAgentTools.value) {
    if (entry.isGroup) {
      expandedToolGroups.value[entry.groupName] = true;
    }
  }
}

function collapseAllToolGroups() {
  expandedToolGroups.value = {};
}

function isGroupExpanded(groupName: string): boolean {
  if (agentToolSearchQuery.value.trim() !== '') {
    return true;
  }
  return !!expandedToolGroups.value[groupName];
}

const displayAgentTools = computed<DisplayToolEntry[]>(() => {
  const tools = filteredAgentTools.value;
  if (toolViewMode.value === 'flat') {
    return tools.map((t) => ({ isGroup: false as const, tool: t }));
  }

  const groupMap: Record<string, AgentToolStat[]> = {};
  const ungrouped: AgentToolStat[] = [];

  for (const t of tools) {
    if (t.server_group) {
      if (!groupMap[t.server_group]) {
        groupMap[t.server_group] = [];
      }
      groupMap[t.server_group].push(t);
    } else {
      ungrouped.push(t);
    }
  }

  const result: DisplayToolEntry[] = [];

  for (const [groupName, groupTools] of Object.entries(groupMap)) {
    const totalCalls = groupTools.reduce((acc, curr) => acc + curr.call_count, 0);
    const cat = groupName.toLowerCase().includes('native') ? 'Native' : (groupTools[0]?.category || 'MCP');
    result.push({
      isGroup: true,
      groupName,
      category: cat,
      callCount: totalCalls,
      tools: groupTools,
    });
  }

  for (const t of ungrouped) {
    result.push({
      isGroup: false,
      tool: t,
    });
  }

  result.sort((a, b) => {
    const countA = a.isGroup ? a.callCount : a.tool.call_count;
    const countB = b.isGroup ? b.callCount : b.tool.call_count;
    return countB - countA;
  });

  return result;
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
    const palette = ['#38bdf8', '#10b981', '#f59e0b', '#a855f7', '#ec4899', '#6366f1', '#14b8a6', '#f43f5e'];

    modelChartInstance = new Chart(modelChartCanvas.value, {
      type: 'doughnut',
      data: {
        labels: modelLabels,
        datasets: [
          {
            data: modelData,
            backgroundColor: palette.slice(0, modelLabels.length),
            borderWidth: 1,
            borderColor: '#1e293b',
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'right',
            labels: {
              color: '#94a3b8',
              font: { family: 'monospace', size: 10 },
              boxWidth: 12,
            },
          },
        },
      },
    });
  }

  // 2. Bar Chart for Daily Timeline
  if (timelineChartCanvas.value && stats.value.daily_metrics) {
    if (timelineChartInstance) timelineChartInstance.destroy();
    const sorted = [...stats.value.daily_metrics].sort((a, b) => a.date.localeCompare(b.date));
    const dates = sorted.map((d) => d.date.slice(5)); // MM-DD
    const inputData = sorted.map((d) => d.input_tokens);
    const cacheData = sorted.map((d) => d.cache_read_tokens);

    timelineChartInstance = new Chart(timelineChartCanvas.value, {
      type: 'bar',
      data: {
        labels: dates,
        datasets: [
          {
            label: 'Cache Read (Hit)',
            data: cacheData,
            backgroundColor: '#10b981',
            borderRadius: 4,
          },
          {
            label: 'Input Mới',
            data: inputData,
            backgroundColor: '#38bdf8',
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
            ticks: { color: '#94a3b8', font: { family: 'monospace', size: 10 } },
            grid: { display: false },
          },
          y: {
            stacked: true,
            ticks: {
              color: '#94a3b8',
              font: { family: 'monospace', size: 10 },
              callback: (val) => formatNum(val as number),
            },
            grid: { color: 'rgba(255,255,255,0.05)' },
          },
        },
        plugins: {
          legend: {
            position: 'top',
            labels: {
              color: '#94a3b8',
              font: { family: 'monospace', size: 11 },
            },
          },
        },
      },
    });
  }
}

watch(activeDashboardTab, async (newTab) => {
  if (newTab === 'overview') {
    await nextTick();
    renderCharts();
  }
});

onMounted(async () => {
  if (!serverState.isConnected) {
    router.push({ name: 'login' });
    return;
  }
  await handleRefresh();
});
</script>

<template>
  <div class="min-h-screen bg-paper text-ink transition-colors pb-16">
    <!-- Top Global Header -->
    <header class="border-b border-rule bg-paper/80 backdrop-blur sticky top-0 z-30">
      <div class="w-full px-4 sm:px-6 lg:px-8 py-3.5 flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl bg-cool/10 border border-cool/20 flex items-center justify-center text-cool">
            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>
            </svg>
          </div>
          <div>
            <h1 class="text-base sm:text-lg font-bold tracking-tight text-ink flex items-center gap-2">
              Antigravity Token &amp; Cost Visualizer
            </h1>
            <p class="text-xs text-ink-dim font-mono truncate max-w-md">
              {{ serverState.endpoint }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2.5">
          <!-- Server Status Pill -->
          <div class="flex items-center gap-2 px-3 py-1 rounded-full text-xs font-mono bg-paper-subtle border border-rule">
            <span class="w-2 h-2 rounded-full bg-hit animate-pulse"></span>
            <span class="text-ink-muted hidden sm:inline">Trực tuyến:</span>
            <span class="font-bold text-ink truncate max-w-[120px]">{{ serverState.endpoint.replace(/^https?:\/\//, '') }}</span>
          </div>

          <!-- Change Server Button -->
          <button
            @click="handleChangeServer"
            class="px-3 py-1.5 rounded-lg text-xs font-mono font-medium border border-rule hover:border-cool/40 hover:bg-cool/10 text-ink-muted hover:text-cool transition cursor-pointer"
            title="Đổi máy chủ khác"
          >
            Đổi Server
          </button>

          <!-- Refresh Button -->
          <button
            @click="handleRefresh"
            :disabled="serverState.loading"
            class="p-2 rounded-lg border border-rule hover:border-cool/40 hover:bg-cool/10 text-ink-muted hover:text-cool transition cursor-pointer disabled:opacity-50"
            title="Tải lại dữ liệu"
          >
            <svg
              class="w-4 h-4"
              :class="{ 'animate-spin': serverState.loading }"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
            >
              <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
            </svg>
          </button>
        </div>
      </div>
    </header>

    <main class="w-full px-4 sm:px-6 lg:px-8 py-6 space-y-6">
      <!-- Loading Skeleton Screen -->
      <DashboardSkeleton v-if="serverState.loading && !stats" />

      <!-- Loaded Dashboard Content -->
      <template v-else-if="stats">
        <!-- 5 Key System KPI Cards -->
        <div class="grid grid-cols-2 lg:grid-cols-5 gap-3.5 sm:gap-4">
          <!-- 1. Cache Hit Tokens -->
          <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow">
            <div class="text-xs text-ink-muted font-medium mb-1">Cache Hit Tokens</div>
            <div class="text-xl sm:text-2xl font-bold font-mono text-hit leading-tight">
              {{ formatNum(stats.total_cache_read) }}
            </div>
            <p class="text-[11px] text-ink-dim mt-2 truncate">
              Đã tái sử dụng từ Context Cache
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
              Thinking CoT + Responses
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

        <!-- Dashboard Sub-Navigation Tabs Bar -->
        <nav class="flex items-center gap-1.5 sm:gap-2 border-b border-rule pb-2 overflow-x-auto no-scrollbar font-mono text-xs">
          <button
            type="button"
            @click="activeDashboardTab = 'overview'"
            class="px-3.5 py-2 rounded-xl transition flex items-center gap-2 cursor-pointer whitespace-nowrap font-medium"
            :class="activeDashboardTab === 'overview' ? 'bg-cool text-paper font-semibold shadow-sm' : 'bg-paper-card border border-rule text-ink-muted hover:text-ink hover:border-cool/40'"
          >
            <span>📊</span>
            <span>Tổng quan hệ thống</span>
          </button>

          <button
            type="button"
            @click="activeDashboardTab = 'projects'"
            class="px-3.5 py-2 rounded-xl transition flex items-center gap-2 cursor-pointer whitespace-nowrap font-medium"
            :class="activeDashboardTab === 'projects' ? 'bg-cool text-paper font-semibold shadow-sm' : 'bg-paper-card border border-rule text-ink-muted hover:text-ink hover:border-cool/40'"
          >
            <span>📁</span>
            <span>Danh sách Dự án</span>
            <span class="px-1.5 py-0.2 rounded-full text-[10px] font-bold" :class="activeDashboardTab === 'projects' ? 'bg-paper/20 text-paper' : 'bg-paper-subtle text-cool'">
              {{ stats.projects?.length || 0 }}
            </span>
          </button>

          <button
            type="button"
            @click="activeDashboardTab = 'models'"
            class="px-3.5 py-2 rounded-xl transition flex items-center gap-2 cursor-pointer whitespace-nowrap font-medium"
            :class="activeDashboardTab === 'models' ? 'bg-cool text-paper font-semibold shadow-sm' : 'bg-paper-card border border-rule text-ink-muted hover:text-ink hover:border-cool/40'"
          >
            <span>✨</span>
            <span>Danh sách Model</span>
            <span class="px-1.5 py-0.2 rounded-full text-[10px] font-bold" :class="activeDashboardTab === 'models' ? 'bg-paper/20 text-paper' : 'bg-paper-subtle text-accent'">
              {{ stats.models?.length || 0 }}
            </span>
          </button>

          <button
            type="button"
            @click="activeDashboardTab = 'tools'"
            class="px-3.5 py-2 rounded-xl transition flex items-center gap-2 cursor-pointer whitespace-nowrap font-medium"
            :class="activeDashboardTab === 'tools' ? 'bg-cool text-paper font-semibold shadow-sm' : 'bg-paper-card border border-rule text-ink-muted hover:text-ink hover:border-cool/40'"
          >
            <span>🛠️</span>
            <span>Công cụ &amp; Môi trường</span>
            <span class="px-1.5 py-0.2 rounded-full text-[10px] font-bold" :class="activeDashboardTab === 'tools' ? 'bg-paper/20 text-paper' : 'bg-paper-subtle text-purple-400'">
              {{ stats.tools?.length || 0 }}
            </span>
          </button>

          <button
            type="button"
            @click="activeDashboardTab = 'sessions'"
            class="px-3.5 py-2 rounded-xl transition flex items-center gap-2 cursor-pointer whitespace-nowrap font-medium"
            :class="activeDashboardTab === 'sessions' ? 'bg-cool text-paper font-semibold shadow-sm' : 'bg-paper-card border border-rule text-ink-muted hover:text-ink hover:border-cool/40'"
          >
            <span>🧭</span>
            <span>Khám phá Hội thoại</span>
            <span class="px-1.5 py-0.2 rounded-full text-[10px] font-bold" :class="activeDashboardTab === 'sessions' ? 'bg-paper/20 text-paper' : 'bg-paper-subtle text-ink-dim'">
              {{ stats.total_sessions || 0 }}
            </span>
          </button>
        </nav>

        <!-- ======================================================== -->
        <!-- TAB 1: 📊 TỔNG QUAN HỆ THỐNG (Charts & Highlights)        -->
        <!-- ======================================================== -->
        <div v-show="activeDashboardTab === 'overview'" class="space-y-6">
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

          <!-- Multi-Environment & Highlights Grid -->
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <!-- Highlight 1: Tools Breakdown -->
            <div class="bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col justify-between">
              <div>
                <div class="flex items-center justify-between mb-2">
                  <h4 class="text-xs font-bold text-ink uppercase tracking-wider font-mono">Công cụ sử dụng</h4>
                  <button @click="activeDashboardTab = 'tools'" class="text-[11px] text-cool hover:underline cursor-pointer">Chi tiết &rarr;</button>
                </div>
                <div class="space-y-2.5 mt-3 font-mono">
                  <div
                    v-for="t in stats.tools || []"
                    :key="t.name"
                    @click="navigateToToolSessions(t.name)"
                    class="p-2.5 rounded-xl bg-paper/60 hover:bg-paper border border-rule flex items-center justify-between cursor-pointer transition text-xs group"
                  >
                    <div class="flex items-center gap-2">
                      <span class="text-base">{{ t.icon === 'desktop' ? '🖥️' : t.icon === 'terminal' ? '⌨️' : '💻' }}</span>
                      <div>
                        <div class="font-medium text-ink group-hover:text-cool">{{ t.name }}</div>
                        <div class="text-[10px] text-ink-dim">{{ t.session_count }} hội thoại</div>
                      </div>
                    </div>
                    <div class="text-right">
                      <div class="font-bold text-ink">{{ formatNum(t.total_tokens) }}</div>
                      <div class="text-[10px] text-hit">{{ formatCurrencyUSD(t.cost_usd) }}</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Highlight 2: Top Projects -->
            <div class="bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col justify-between">
              <div>
                <div class="flex items-center justify-between mb-2">
                  <h4 class="text-xs font-bold text-ink uppercase tracking-wider font-mono">Top Dự án tiêu thụ</h4>
                  <button @click="activeDashboardTab = 'projects'" class="text-[11px] text-cool hover:underline cursor-pointer">Chi tiết &rarr;</button>
                </div>
                <div class="space-y-2.5 mt-3 font-mono">
                  <div
                    v-for="p in (stats.projects || []).slice(0, 3)"
                    :key="p.name"
                    @click="navigateToProjectSessions(p.name)"
                    class="p-2.5 rounded-xl bg-paper/60 hover:bg-paper border border-rule flex items-center justify-between cursor-pointer transition text-xs group"
                  >
                    <div class="min-w-0 pr-2">
                      <div class="font-medium text-ink group-hover:text-cool truncate">{{ p.name }}</div>
                      <div class="text-[10px] text-ink-dim">{{ p.session_count }} hội thoại • {{ (p.overall_cache_hit_rate || 0).toFixed(1) }}% hit</div>
                    </div>
                    <div class="text-right flex-shrink-0">
                      <div class="font-bold text-ink">{{ formatNum(p.total_tokens) }}</div>
                      <div class="text-[10px] text-hit">{{ formatCurrencyUSD(p.total_cost_usd) }}</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Highlight 3: Top Agent Tools -->
            <div class="bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col justify-between">
              <div>
                <div class="flex items-center justify-between mb-2">
                  <h4 class="text-xs font-bold text-ink uppercase tracking-wider font-mono">Top Lệnh Agent gọi</h4>
                  <button @click="activeDashboardTab = 'tools'" class="text-[11px] text-cool hover:underline cursor-pointer">Chi tiết &rarr;</button>
                </div>
                <div class="space-y-2 mt-3 font-mono text-xs">
                  <div
                    v-for="at in (stats.agent_tools || []).slice(0, 4)"
                    :key="at.name"
                    class="flex items-center justify-between py-1.5 border-b border-rule/40 last:border-0"
                  >
                    <div class="flex items-center gap-1.5 truncate">
                      <span class="text-ink-dim">🔧</span>
                      <span class="font-semibold text-ink truncate">{{ at.name }}</span>
                    </div>
                    <span class="badge-cool px-2 py-0.5 rounded text-[11px] font-bold">
                      {{ at.call_count.toLocaleString('vi-VN') }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- TAB 2: 📁 DANH SÁCH DỰ ÁN & THỐNG KÊ CHI TIẾT             -->
        <!-- ======================================================== -->
        <div v-show="activeDashboardTab === 'projects'" class="space-y-4">
          <!-- Projects Toolbar -->
          <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 shadow flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h2 class="text-base font-bold text-ink flex items-center gap-2">
                📁 Danh sách &amp; Thống kê Dự án
                <span class="badge-cool px-2 py-0.5 rounded-full text-xs font-mono font-medium">
                  {{ sortedProjectsList.length }} dự án
                </span>
              </h2>
              <p class="text-xs text-ink-dim mt-1">
                Phân tích tiêu thụ token, hiệu suất Cache Hit và chi phí USD/VND theo từng dự án
              </p>
            </div>

            <!-- Search & Sort Controls -->
            <div class="flex flex-wrap items-center gap-2.5">
              <input
                v-model="projectSearchQuery"
                type="text"
                placeholder="Tìm dự án hoặc đường dẫn..."
                class="px-3 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink placeholder-ink-dim font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool w-56 transition"
              />
              <select
                v-model="projectSortBy"
                class="px-3 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink font-mono focus:outline-none focus:border-cool cursor-pointer"
              >
                <option value="tokens">Nhiều Token nhất</option>
                <option value="cost">Chi phí cao nhất</option>
                <option value="cache">Cache Hit cao nhất</option>
                <option value="sessions">Nhiều hội thoại nhất</option>
                <option value="recent">Mới hoạt động nhất</option>
                <option value="name">Tên A-Z</option>
              </select>
            </div>
          </div>

          <!-- Projects Grid Cards -->
          <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            <div
              v-for="p in sortedProjectsList"
              :key="p.name"
              class="bg-paper-card border border-rule rounded-2xl p-5 shadow hover:border-cool/40 transition flex flex-col justify-between group"
            >
              <div>
                <!-- Project Name & Path -->
                <div class="flex items-start justify-between gap-2 mb-2">
                  <h3 class="font-bold text-sm text-ink group-hover:text-cool transition font-mono truncate" :title="p.name">
                    {{ p.name }}
                  </h3>
                  <span class="badge-cool px-2 py-0.5 rounded text-[10px] font-mono font-bold flex-shrink-0">
                    {{ p.session_count }} hội thoại
                  </span>
                </div>
                <div class="text-[11px] text-ink-dim font-mono truncate mb-4" :title="p.workspace_path">
                  {{ p.workspace_path || 'Không có đường dẫn cục bộ' }}
                </div>

                <!-- KPI Mini-Grid -->
                <div class="grid grid-cols-3 gap-2 bg-paper/60 p-3 rounded-xl border border-rule/60 font-mono mb-4 text-xs">
                  <div>
                    <div class="text-[10px] text-ink-dim">Tổng Tokens</div>
                    <div class="font-bold text-ink">{{ formatNum(p.total_tokens) }}</div>
                  </div>
                  <div>
                    <div class="text-[10px] text-ink-dim">Cache Hit</div>
                    <div class="font-bold text-hit">{{ (p.overall_cache_hit_rate || 0).toFixed(1) }}%</div>
                  </div>
                  <div>
                    <div class="text-[10px] text-ink-dim">Chi phí</div>
                    <div class="font-bold text-cool truncate">{{ formatCurrencyUSD(p.total_cost_usd) }}</div>
                  </div>
                </div>

                <!-- Token Breakdown Bar -->
                <div class="space-y-1 mb-4">
                  <div class="flex justify-between text-[10px] font-mono text-ink-dim">
                    <span>Phân bổ Tokens</span>
                    <span>{{ formatNum(p.total_cache_read) }} cache / {{ formatNum(p.total_input_tokens) }} input</span>
                  </div>
                  <div class="h-2 rounded-full bg-paper border border-rule overflow-hidden flex">
                    <div
                      class="bg-hit h-full"
                      :style="{ width: `${p.total_tokens > 0 ? (p.total_cache_read / p.total_tokens) * 100 : 0}%` }"
                      title="Cache Read Tokens"
                    ></div>
                    <div
                      class="bg-accent h-full"
                      :style="{ width: `${p.total_tokens > 0 ? (p.total_input_tokens / p.total_tokens) * 100 : 0}%` }"
                      title="Input Tokens"
                    ></div>
                    <div
                      class="bg-output h-full"
                      :style="{ width: `${p.total_tokens > 0 ? (p.total_output / p.total_tokens) * 100 : 0}%` }"
                      title="Output Tokens"
                    ></div>
                  </div>
                </div>

                <!-- Models Badges in this Project -->
                <div class="flex flex-wrap gap-1.5 mb-4">
                  <span
                    v-for="m in (p.models || []).slice(0, 3)"
                    :key="m.model_name"
                    class="badge-subtle px-2 py-0.5 rounded text-[10px] font-mono truncate max-w-[160px]"
                  >
                    {{ m.model_name }}
                  </span>
                  <span v-if="(p.models || []).length > 3" class="text-[10px] font-mono text-ink-dim self-center">
                    +{{ p.models.length - 3 }} model
                  </span>
                </div>
              </div>

              <!-- Footer with Action Button -->
              <div class="pt-3 border-t border-rule/60 flex items-center justify-between text-xs font-mono">
                <span class="text-[10px] text-ink-dim">
                  {{ p.last_modified ? new Date(p.last_modified).toLocaleDateString('vi-VN') : 'N/A' }}
                </span>
                <button
                  type="button"
                  @click="navigateToProjectSessions(p.name)"
                  class="text-cool hover:text-cool-hover font-semibold flex items-center gap-1 cursor-pointer transition"
                >
                  <span>Xem hội thoại</span>
                  <span>&rarr;</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- TAB 3: ✨ DANH SÁCH MODEL & CHI PHÍ                       -->
        <!-- ======================================================== -->
        <div v-show="activeDashboardTab === 'models'" class="space-y-4">
          <!-- Models Toolbar -->
          <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 shadow flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h2 class="text-base font-bold text-ink flex items-center gap-2">
                ✨ Danh sách &amp; Ma trận Model AI
                <span class="badge-accent px-2 py-0.5 rounded-full text-xs font-mono font-medium">
                  {{ sortedModelsList.length }} mô hình
                </span>
              </h2>
              <p class="text-xs text-ink-dim mt-1">
                Thống kê chi tiết tiêu thụ token, số bước thực thi, tỷ lệ Cache Read và tổng chi phí của từng model
              </p>
            </div>

            <!-- Search & Sort Controls -->
            <div class="flex flex-wrap items-center gap-2.5">
              <input
                v-model="modelSearchQuery"
                type="text"
                placeholder="Tìm tên mô hình..."
                class="px-3 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink placeholder-ink-dim font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool w-52 transition"
              />
              <select
                v-model="modelSortBy"
                class="px-3 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink font-mono focus:outline-none focus:border-cool cursor-pointer"
              >
                <option value="tokens">Nhiều Token nhất</option>
                <option value="cost">Chi phí cao nhất</option>
                <option value="cache">Cache Hit cao nhất</option>
                <option value="steps">Nhiều bước nhất</option>
                <option value="name">Tên Model A-Z</option>
              </select>
            </div>
          </div>

          <!-- Models Table -->
          <div class="bg-paper-card border border-rule rounded-2xl shadow overflow-hidden">
            <div class="overflow-x-auto">
              <table class="w-full text-left text-xs font-mono">
                <thead class="bg-paper/80 border-b border-rule text-ink-dim uppercase tracking-wider text-[11px]">
                  <tr>
                    <th class="py-3.5 px-4 font-semibold">Tên Mô hình (Model)</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Số Bước</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Tổng Tokens</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Cache Hit %</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Input Mới</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Output Tokens</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Chi phí USD</th>
                    <th class="py-3.5 px-4 font-semibold text-right">Chi phí VNĐ</th>
                    <th class="py-3.5 px-4 font-semibold text-center">Hành động</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-rule/60">
                  <tr
                    v-for="m in sortedModelsList"
                    :key="m.model_name"
                    class="hover:bg-paper-subtle/70 transition"
                  >
                    <td class="py-3.5 px-4 font-semibold text-ink flex items-center gap-2">
                      <span
                        class="w-2.5 h-2.5 rounded-full flex-shrink-0"
                        :class="m.model_name.includes('Claude') ? 'bg-purple-400' : m.model_name.includes('GPT') ? 'bg-emerald-400' : 'bg-cool'"
                      ></span>
                      <span class="truncate max-w-xs">{{ m.model_name }}</span>
                    </td>
                    <td class="py-3.5 px-4 text-right text-ink-muted">
                      {{ m.step_count.toLocaleString('vi-VN') }}
                    </td>
                    <td class="py-3.5 px-4 text-right font-bold text-ink">
                      {{ formatNum(m.total_tokens) }}
                    </td>
                    <td class="py-3.5 px-4 text-right font-bold" :class="m.overall_cache_hit_rate > 80 ? 'text-hit' : 'text-cool'">
                      {{ (m.overall_cache_hit_rate || 0).toFixed(1) }}%
                    </td>
                    <td class="py-3.5 px-4 text-right text-accent font-medium">
                      {{ formatNum(m.input_tokens) }}
                    </td>
                    <td class="py-3.5 px-4 text-right text-output font-medium">
                      {{ formatNum(m.output_tokens) }}
                    </td>
                    <td class="py-3.5 px-4 text-right font-bold text-hit">
                      {{ formatCurrencyUSD(m.cost_usd) }}
                    </td>
                    <td class="py-3.5 px-4 text-right text-ink-dim">
                      {{ formatCurrencyVND(m.cost_vnd) }}
                    </td>
                    <td class="py-3.5 px-4 text-center">
                      <button
                        type="button"
                        @click="navigateToModelSessions(m.model_name)"
                        class="px-2.5 py-1 rounded bg-cool/10 hover:bg-cool text-cool hover:text-paper font-semibold transition cursor-pointer text-[11px]"
                      >
                        Lọc hội thoại
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- TAB 4: 🛠️ CÔNG CỤ & MÔI TRƯỜNG (App, CLI, IDE & Agent Tools) -->
        <!-- ======================================================== -->
        <div v-show="activeDashboardTab === 'tools'" class="space-y-6">
          <!-- Section 1: Antigravity Client Environments -->
          <div class="space-y-4">
            <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 shadow flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <h2 class="text-base font-bold text-ink flex items-center gap-2">
                  🛠️ Môi trường &amp; Công cụ Antigravity
                  <span class="badge-cool px-2 py-0.5 rounded-full text-xs font-mono font-medium">
                    {{ stats.tools?.length || 0 }} môi trường
                  </span>
                </h2>
                <p class="text-xs text-ink-dim mt-1">
                  So sánh trực tiếp giữa Antigravity App (Desktop), Antigravity CLI và Antigravity IDE
                </p>
              </div>
            </div>

            <!-- 3 Comparative Tool Cards -->
            <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
              <div
                v-for="tool in stats.tools || []"
                :key="tool.name"
                class="bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col justify-between hover:border-cool/40 transition group"
              >
                <div>
                  <div class="flex items-center justify-between gap-2 mb-3">
                    <div class="flex items-center gap-2.5">
                      <span class="text-2xl">{{ tool.icon === 'desktop' ? '🖥️' : tool.icon === 'terminal' ? '⌨️' : '💻' }}</span>
                      <div>
                        <h3 class="font-bold text-base text-ink group-hover:text-cool transition font-mono">
                          {{ tool.name }}
                        </h3>
                        <span class="text-[10px] font-mono text-ink-dim truncate block max-w-[200px]" :title="tool.data_dir">
                          {{ tool.data_dir }}
                        </span>
                      </div>
                    </div>
                    <span
                      class="px-2 py-0.5 rounded-full text-[10px] font-mono font-bold"
                      :class="tool.session_count > 0 ? 'bg-hit/15 text-hit border border-hit/30' : 'bg-paper-subtle text-ink-dim'"
                    >
                      {{ tool.session_count > 0 ? 'Hoạt động' : 'Sẵn sàng' }}
                    </span>
                  </div>

                  <!-- Metrics List -->
                  <div class="space-y-2.5 bg-paper/60 p-4 rounded-xl border border-rule/60 font-mono text-xs mb-4">
                    <div class="flex justify-between items-center">
                      <span class="text-ink-dim">Tổng số hội thoại:</span>
                      <span class="font-bold text-ink">{{ tool.session_count }}</span>
                    </div>
                    <div class="flex justify-between items-center">
                      <span class="text-ink-dim">Tổng Token tiêu thụ:</span>
                      <span class="font-bold text-ink">{{ formatNum(tool.total_tokens) }}</span>
                    </div>
                    <div class="flex justify-between items-center">
                      <span class="text-ink-dim">Tỷ lệ Cache Hit:</span>
                      <span class="font-bold text-hit">{{ (tool.cache_hit_rate || 0).toFixed(1) }}%</span>
                    </div>
                    <div class="flex justify-between items-center">
                      <span class="text-ink-dim">Chi phí ước tính:</span>
                      <span class="font-bold text-cool">{{ formatCurrencyUSD(tool.cost_usd) }}</span>
                    </div>
                    <div class="flex justify-between items-center">
                      <span class="text-ink-dim">Dự án kết nối:</span>
                      <span class="font-bold text-accent">{{ tool.active_projects }} dự án</span>
                    </div>
                    <div class="flex justify-between items-center text-[10px]">
                      <span class="text-ink-dim">Hoạt động cuối:</span>
                      <span class="text-ink-muted truncate max-w-[140px]">{{ tool.last_activity || 'Chưa ghi nhận' }}</span>
                    </div>
                  </div>
                </div>

                <!-- Card Action Button -->
                <button
                  type="button"
                  @click="navigateToToolSessions(tool.name)"
                  class="w-full py-2 rounded-xl bg-cool/10 hover:bg-cool text-cool hover:text-paper font-semibold font-mono text-xs transition cursor-pointer flex items-center justify-center gap-1.5"
                >
                  <span>Xem {{ tool.session_count }} hội thoại của {{ tool.name }}</span>
                  <span>&rarr;</span>
                </button>
              </div>
            </div>
          </div>

          <!-- Section 2: Agent Tools Execution Analytics -->
          <div class="space-y-4">
            <div class="bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 shadow flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <h3 class="text-sm font-bold text-ink flex items-center gap-2">
                  ⚙️ Lệnh &amp; Công cụ AI Agent đã thực thi (Agent Tools)
                  <span class="badge-cool px-2 py-0.5 rounded-full text-xs font-mono font-medium">
                    {{ totalAgentToolCalls.toLocaleString('vi-VN') }} lượt gọi
                  </span>
                </h3>
                <p class="text-xs text-ink-dim mt-1">
                  Thống kê tần suất gọi các công cụ hệ thống (run_command, view_file, grep_search, edit_file, MCP...)
                </p>
              </div>

              <!-- Search & Filter Category & Group Controls -->
              <div class="flex flex-wrap items-center gap-2.5">
                <!-- View Mode Toggle -->
                <div class="flex rounded-lg border border-rule overflow-hidden bg-paper shadow-sm">
                  <button
                    type="button"
                    @click="toolViewMode = 'grouped'"
                    class="px-2.5 py-1.5 text-xs font-mono transition flex items-center gap-1 cursor-pointer"
                    :class="toolViewMode === 'grouped' ? 'bg-cool text-paper font-semibold' : 'text-ink-muted hover:text-ink'"
                    title="Nhóm theo MCP server / dịch vụ"
                  >
                    <span>📁 Nhóm</span>
                  </button>
                  <button
                    type="button"
                    @click="toolViewMode = 'flat'"
                    class="px-2.5 py-1.5 text-xs font-mono transition flex items-center gap-1 cursor-pointer"
                    :class="toolViewMode === 'flat' ? 'bg-cool text-paper font-semibold' : 'text-ink-muted hover:text-ink'"
                    title="Danh sách phẳng từng lệnh"
                  >
                    <span>📄 Phẳng</span>
                  </button>
                </div>

                <div v-if="toolViewMode === 'grouped'" class="flex items-center gap-1.5">
                  <button
                    type="button"
                    @click="expandAllToolGroups"
                    class="px-2 py-1 rounded bg-paper-subtle hover:bg-paper text-ink-dim hover:text-ink text-[11px] font-mono border border-rule transition cursor-pointer"
                    title="Mở rộng tất cả nhóm MCP"
                  >
                    Mở tất cả
                  </button>
                  <button
                    type="button"
                    @click="collapseAllToolGroups"
                    class="px-2 py-1 rounded bg-paper-subtle hover:bg-paper text-ink-dim hover:text-ink text-[11px] font-mono border border-rule transition cursor-pointer"
                    title="Thu gọn tất cả nhóm MCP"
                  >
                    Thu gọn
                  </button>
                </div>

                <input
                  v-model="agentToolSearchQuery"
                  type="text"
                  placeholder="Tìm tên lệnh tool..."
                  class="px-3 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink placeholder-ink-dim font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool w-44 sm:w-48 transition"
                />
                <select
                  v-model="selectedAgentToolCategory"
                  class="px-3 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink font-mono focus:outline-none focus:border-cool cursor-pointer"
                >
                  <option v-for="cat in agentToolCategories" :key="cat" :value="cat">
                    {{ cat === 'all' ? 'Tất cả phân loại' : cat }}
                  </option>
                </select>
              </div>
            </div>

            <!-- Agent Tools Table -->
            <div class="bg-paper-card border border-rule rounded-2xl shadow overflow-hidden">
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs font-mono">
                  <thead class="bg-paper/80 border-b border-rule text-ink-dim uppercase tracking-wider text-[11px]">
                    <tr>
                      <th class="py-3 px-4 font-semibold">Tên Tool</th>
                      <th class="py-3 px-4 font-semibold">Phân loại</th>
                      <th class="py-3 px-4 font-semibold text-right">Số lượt gọi</th>
                      <th class="py-3 px-4 font-semibold">Tỷ lệ tần suất</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-rule/60">
                    <template v-for="entry in displayAgentTools" :key="entry.isGroup ? entry.groupName : entry.tool.name">
                      <!-- Group Header Row -->
                      <tr
                        v-if="entry.isGroup"
                        @click="toggleToolGroup(entry.groupName)"
                        class="bg-cool/5 hover:bg-cool/10 transition cursor-pointer select-none group"
                      >
                        <td class="py-2.5 px-4 font-bold text-ink flex items-center gap-2">
                          <span
                            class="text-cool text-xs transform transition-transform"
                            :class="{ 'rotate-90': isGroupExpanded(entry.groupName) }"
                          >
                            ▶
                          </span>
                          <span class="text-cool font-mono text-sm">{{ entry.groupName.includes('Native') ? '⚡' : '🧩' }}</span>
                          <span class="text-cool font-bold">{{ entry.groupName }}</span>
                          <span class="badge-cool px-2 py-0.5 rounded-full text-[10px] font-mono font-medium">
                            {{ entry.tools.length }} lệnh con
                          </span>
                        </td>
                        <td class="py-2.5 px-4">
                          <span class="badge-subtle px-2 py-0.5 rounded text-[10px]">
                            {{ entry.category }}
                          </span>
                        </td>
                        <td class="py-2.5 px-4 text-right font-bold text-cool">
                          {{ entry.callCount.toLocaleString('vi-VN') }}
                        </td>
                        <td class="py-2.5 px-4 w-48">
                          <div class="flex items-center gap-2">
                            <div class="h-2 rounded-full bg-paper border border-rule overflow-hidden flex-1">
                              <div
                                class="bg-cool h-full"
                                :style="{ width: `${totalAgentToolCalls > 0 ? (entry.callCount / totalAgentToolCalls) * 100 : 0}%` }"
                              ></div>
                            </div>
                            <span class="text-[10px] text-cool font-mono w-10 text-right font-bold">
                              {{ totalAgentToolCalls > 0 ? ((entry.callCount / totalAgentToolCalls) * 100).toFixed(1) : 0 }}%
                            </span>
                          </div>
                        </td>
                      </tr>

                      <!-- Sub-rows when group is expanded -->
                      <template v-if="entry.isGroup && isGroupExpanded(entry.groupName)">
                        <tr
                          v-for="sub in entry.tools"
                          :key="sub.name"
                          class="bg-paper-subtle/30 hover:bg-paper-subtle/70 transition"
                        >
                          <td class="py-2 px-4 pl-8 sm:pl-10 text-ink flex items-center gap-2">
                            <span class="text-cool/60 text-xs">↳</span>
                            <span class="text-ink-muted text-xs">🔧</span>
                            <span class="font-mono text-xs text-ink-muted">{{ sub.name }}</span>
                          </td>
                          <td class="py-2 px-4">
                            <span class="badge-subtle px-1.5 py-0.5 rounded text-[9px] text-ink-dim">
                              {{ sub.category }}
                            </span>
                          </td>
                          <td class="py-2 px-4 text-right text-xs font-mono text-ink-muted">
                            {{ sub.call_count.toLocaleString('vi-VN') }}
                          </td>
                          <td class="py-2 px-4 w-48">
                            <div class="flex items-center gap-2">
                              <div class="h-1.5 rounded-full bg-paper border border-rule/50 overflow-hidden flex-1">
                                <div
                                  class="bg-cool/60 h-full"
                                  :style="{ width: `${entry.callCount > 0 ? (sub.call_count / entry.callCount) * 100 : 0}%` }"
                                ></div>
                              </div>
                              <span class="text-[9px] text-ink-dim font-mono w-10 text-right">
                                {{ entry.callCount > 0 ? ((sub.call_count / entry.callCount) * 100).toFixed(1) : 0 }}%
                              </span>
                            </div>
                          </td>
                        </tr>
                      </template>

                      <!-- Standalone Tool Row -->
                      <tr
                        v-if="!entry.isGroup"
                        class="hover:bg-paper-subtle/70 transition"
                      >
                        <td class="py-2.5 px-4 font-bold text-ink flex items-center gap-2">
                          <span class="text-cool">🔧</span>
                          <span>{{ entry.tool.name }}</span>
                        </td>
                        <td class="py-2.5 px-4">
                          <span class="badge-subtle px-2 py-0.5 rounded text-[10px]">
                            {{ entry.tool.category }}
                          </span>
                        </td>
                        <td class="py-2.5 px-4 text-right font-bold text-ink">
                          {{ entry.tool.call_count.toLocaleString('vi-VN') }}
                        </td>
                        <td class="py-2.5 px-4 w-48">
                          <div class="flex items-center gap-2">
                            <div class="h-2 rounded-full bg-paper border border-rule overflow-hidden flex-1">
                              <div
                                class="bg-cool h-full"
                                :style="{ width: `${totalAgentToolCalls > 0 ? (entry.tool.call_count / totalAgentToolCalls) * 100 : 0}%` }"
                              ></div>
                            </div>
                            <span class="text-[10px] text-ink-dim font-mono w-10 text-right">
                              {{ totalAgentToolCalls > 0 ? ((entry.tool.call_count / totalAgentToolCalls) * 100).toFixed(1) : 0 }}%
                            </span>
                          </div>
                        </td>
                      </tr>
                    </template>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>

        <!-- ======================================================== -->
        <!-- TAB 5: 🧭 KHÁM PHÁ HỘI THOẠI (Sessions Explorer)          -->
        <!-- ======================================================== -->
        <div v-show="activeDashboardTab === 'sessions'" class="space-y-4">
          <!-- Sessions Explorer Container -->
          <div class="bg-paper-card border border-rule rounded-2xl shadow overflow-hidden">
            <!-- Explorer Header: Row 1 (Title, Counts & View Mode Switch) -->
            <div class="p-4 sm:p-5 border-b border-rule/60 flex flex-col md:flex-row md:items-center justify-between gap-4">
              <div class="min-w-0">
                <div class="flex items-center gap-2.5 flex-wrap">
                  <h2 class="text-sm sm:text-base font-bold text-ink whitespace-nowrap">Khám phá Phiên làm việc (Sessions Explorer)</h2>
                  <span class="text-xs badge-cool px-2.5 py-0.5 rounded-full font-mono font-medium shrink-0">
                    {{ viewMode === 'project' ? `${filteredProjects.length} / ${stats.projects?.length || 0} dự án` : `${allFlatSessions.length} hội thoại` }}
                  </span>
                  <span v-if="isAnyFilterActive" class="text-[11px] badge-accent px-2 py-0.5 rounded-full font-mono font-medium shrink-0">
                    Đang lọc
                  </span>
                </div>
                <p class="text-xs text-ink-dim mt-1">
                  Phân cấp theo Dự án &rarr; Hội thoại &rarr; Nhóm Model &rarr; Chi tiết Chat Stream
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
              <div class="relative flex-1 min-w-[180px] sm:min-w-[220px] max-w-sm">
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

              <!-- Tool/Client Environment Searchable Select -->
              <SearchableSelect
                v-model="selectedToolFilter"
                :options="toolSelectOptions"
                placeholder="Tất cả Công cụ"
                search-placeholder="Tìm công cụ..."
                icon="filter"
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
                class="px-2.5 py-1.5 rounded-lg text-xs font-mono font-medium text-ink-dim hover:text-ink hover:bg-paper border border-rule/60 hover:border-rule flex items-center gap-1.5 transition cursor-pointer"
                title="Xóa toàn bộ các bộ lọc đang chọn"
              >
                <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="18" y1="6" x2="6" y2="18"/>
                  <line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
                <span>Đặt lại</span>
              </button>
            </div>

            <!-- Project-based View Mode -->
            <div v-if="viewMode === 'project'" class="divide-y divide-rule">
              <div v-if="filteredProjects.length === 0" class="p-8 text-center text-ink-dim text-xs font-mono">
                Không tìm thấy dự án hoặc phiên làm việc nào phù hợp với bộ lọc.
              </div>

              <div v-for="p in filteredProjects" :key="p.name" class="transition">
                <!-- Project Row -->
                <div
                  @click="toggleProject(p.name)"
                  class="p-4 bg-paper-subtle/50 hover:bg-paper-subtle cursor-pointer flex items-center justify-between gap-4 transition select-none"
                >
                  <div class="flex items-center gap-3 min-w-0">
                    <span class="text-cool text-xs transform transition-transform" :class="{ '-rotate-90': collapsedProjects[p.name] }">
                      &#9660;
                    </span>
                    <div class="min-w-0">
                      <div class="flex items-center gap-2 flex-wrap">
                        <span class="font-bold text-sm text-ink truncate font-mono">{{ p.name }}</span>
                        <span class="text-xs badge-cool px-2 py-0.5 rounded-full font-mono font-medium">
                          {{ p.sessions.length }} hội thoại
                        </span>
                      </div>
                      <div class="text-[11px] text-ink-dim font-mono truncate" :title="p.workspace_path">
                        {{ p.workspace_path || 'Global / Other' }}
                      </div>
                    </div>
                  </div>

                  <div class="flex items-center gap-4 text-xs font-mono text-right flex-shrink-0">
                    <div class="hidden sm:block">
                      <div class="font-bold text-ink">{{ formatNum(p.total_tokens) }} tokens</div>
                      <div class="text-[11px] text-hit">
                        Hit: {{ (p.overall_cache_hit_rate || 0).toFixed(1) }}%
                      </div>
                    </div>
                    <div class="text-hit font-bold text-sm">
                      {{ formatCurrencyUSD(p.total_cost_usd) }}
                    </div>
                  </div>
                </div>

                <!-- Sessions List under Project -->
                <div v-if="!collapsedProjects[p.name]" class="bg-paper/40 divide-y divide-rule/60 border-t border-rule/60">
                  <template v-for="s in p.sessions" :key="s.id">
                    <div
                      :data-session-id="s.id"
                      @click="openSessionDetail(s.id)"
                      class="session-row px-6 sm:px-8 py-3 hover:bg-paper-subtle/70 cursor-pointer flex items-center justify-between gap-4 transition text-xs group"
                    >
                      <div class="min-w-0">
                        <div class="flex items-center gap-2 flex-wrap">
                          <!-- Subagent Badge -->
                          <span
                            v-if="s.is_subagent"
                            class="text-[10px] px-1.5 py-0.5 rounded font-mono flex-shrink-0 bg-purple-500/15 text-purple-300 border border-purple-500/30"
                            :title="s.parent_conversation_id ? `Subagent của ${s.parent_conversation_id.slice(0, 8)}` : 'Subagent'"
                          >
                            Subagent<span v-if="s.agent_name">: {{ s.agent_name }}</span>
                          </span>

                          <!-- Client Tool Badge -->
                          <span
                            v-if="s.client_tool"
                            class="text-[10px] px-1.5 py-0.5 rounded font-mono flex-shrink-0 bg-blue-500/10 text-blue-300 border border-blue-500/20"
                          >
                            {{ s.client_tool === 'Antigravity CLI' ? '⌨️ CLI' : s.client_tool === 'Antigravity IDE' ? '💻 IDE' : '🖥️ App' }}
                          </span>

                          <!-- Session Title -->
                          <span class="font-medium text-ink group-hover:text-cool transition truncate font-mono">
                            {{ s.title }}
                          </span>

                          <!-- Short ID -->
                          <span class="text-[10px] badge-subtle px-1.5 py-0.5 rounded font-mono flex-shrink-0">
                            {{ s.id.substring(0, 8) }}
                          </span>

                          <!-- Subagents Count & Expand Toggle Button -->
                          <button
                            v-if="subagentsByParentId[s.id]?.length"
                            type="button"
                            @click.stop="toggleSubagents(s.id)"
                            class="text-[10px] px-2 py-0.5 rounded font-mono font-semibold flex items-center gap-1 transition cursor-pointer"
                            :class="expandedSubagentSessions[s.id] ? 'bg-purple-500/25 text-purple-200 border border-purple-500/50' : 'bg-purple-500/10 text-purple-300 hover:bg-purple-500/20 border border-purple-500/30'"
                            title="Xem danh sách subagents của phiên này"
                          >
                            <span>🤖 {{ subagentsByParentId[s.id].length }} subagents</span>
                            <span class="transform transition-transform text-[8px]" :class="{ 'rotate-180': expandedSubagentSessions[s.id] }">▼</span>
                          </button>
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

                    <!-- Subagents Nested Drawer List -->
                    <div
                      v-if="expandedSubagentSessions[s.id] && subagentsByParentId[s.id]"
                      class="bg-purple-950/20 border-y border-purple-500/20 pl-8 sm:pl-12 pr-6 sm:pr-8 py-2 divide-y divide-purple-500/10 space-y-1.5"
                    >
                      <div
                        v-for="sub in subagentsByParentId[s.id]"
                        :key="sub.id"
                        @click="openSessionDetail(sub.id)"
                        class="py-2 hover:bg-paper-subtle/50 rounded-lg px-3 -mx-3 cursor-pointer flex items-center justify-between gap-4 transition text-xs group"
                      >
                        <div class="min-w-0">
                          <div class="flex items-center gap-2 flex-wrap">
                            <span class="text-cool text-xs">↳</span>
                            <span class="text-[10px] px-1.5 py-0.5 rounded font-mono bg-purple-500/20 text-purple-300 border border-purple-500/30">
                              Subagent: {{ sub.agent_name || 'agent' }}
                            </span>
                            <span class="font-mono text-ink group-hover:text-cool transition truncate font-medium">
                              {{ sub.title }}
                            </span>
                            <span class="text-[10px] badge-subtle px-1.5 py-0.5 rounded font-mono">
                              {{ sub.id.substring(0, 8) }}
                            </span>
                          </div>
                          <div class="flex items-center gap-3 text-[11px] text-ink-dim mt-1 font-mono pl-4">
                            <span>{{ new Date(sub.last_modified).toLocaleString('vi-VN') }}</span>
                            <span>• {{ sub.step_count }} bước</span>
                            <span>• {{ sub.model_name }}</span>
                          </div>
                        </div>
                        <div class="flex items-center gap-3 font-mono text-xs flex-shrink-0">
                          <span class="text-ink-muted hidden sm:inline">{{ formatNum(sub.total_tokens) }}</span>
                          <span class="px-2 py-0.5 rounded text-[10px] font-bold" :class="sub.cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'">
                            {{ sub.cache_hit_rate.toFixed(1) }}%
                          </span>
                          <span class="text-hit font-bold">
                            {{ formatCurrencyUSD(sub.cost_usd) }}
                          </span>
                          <span class="text-cool group-hover:translate-x-1 transition-transform">
                            &rarr;
                          </span>
                        </div>
                      </div>
                    </div>
                  </template>
                </div>
              </div>
            </div>

            <!-- Flat Sessions View Mode -->
            <div v-else class="divide-y divide-rule">
              <template v-for="s in allFlatSessions" :key="s.id">
                <div
                  :data-session-id="s.id"
                  @click="openSessionDetail(s.id)"
                  class="session-row p-4 hover:bg-paper-subtle/70 cursor-pointer flex items-center justify-between gap-4 transition text-xs group"
                >
                  <div class="min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <!-- Subagent Badge -->
                      <span
                        v-if="s.is_subagent"
                        class="text-[10px] px-1.5 py-0.5 rounded font-mono flex-shrink-0 bg-purple-500/15 text-purple-300 border border-purple-500/30"
                        :title="s.parent_conversation_id ? `Subagent của ${s.parent_conversation_id.slice(0, 8)}` : 'Subagent'"
                      >
                        Subagent<span v-if="s.agent_name">: {{ s.agent_name }}</span>
                      </span>

                      <!-- Client Tool Badge -->
                      <span
                        v-if="s.client_tool"
                        class="text-[10px] px-1.5 py-0.5 rounded font-mono flex-shrink-0 bg-blue-500/10 text-blue-300 border border-blue-500/20"
                      >
                        {{ s.client_tool === 'Antigravity CLI' ? '⌨️ CLI' : s.client_tool === 'Antigravity IDE' ? '💻 IDE' : '🖥️ App' }}
                      </span>

                      <!-- Title -->
                      <span class="font-semibold text-ink group-hover:text-cool transition truncate font-mono">
                        {{ s.title }}
                      </span>

                      <!-- Short ID -->
                      <span class="text-[10px] badge-subtle px-1.5 py-0.5 rounded font-mono flex-shrink-0">
                        {{ s.id.substring(0, 8) }}
                      </span>

                      <!-- Subagents Count & Expand Toggle Button -->
                      <button
                        v-if="subagentsByParentId[s.id]?.length"
                        type="button"
                        @click.stop="toggleSubagents(s.id)"
                        class="text-[10px] px-2 py-0.5 rounded font-mono font-semibold flex items-center gap-1 transition cursor-pointer"
                        :class="expandedSubagentSessions[s.id] ? 'bg-purple-500/25 text-purple-200 border border-purple-500/50' : 'bg-purple-500/10 text-purple-300 hover:bg-purple-500/20 border border-purple-500/30'"
                        title="Xem danh sách subagents của phiên này"
                      >
                        <span>🤖 {{ subagentsByParentId[s.id].length }} subagents</span>
                        <span class="transform transition-transform text-[8px]" :class="{ 'rotate-180': expandedSubagentSessions[s.id] }">▼</span>
                      </button>
                    </div>
                    <div class="flex items-center gap-3 text-[11px] text-ink-dim mt-1 font-mono">
                      <span class="text-cool font-medium">{{ s.project_name || 'Global' }}</span>
                      <span>• {{ new Date(s.last_modified).toLocaleString('vi-VN') }}</span>
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

                <!-- Subagents Nested Drawer List (Flat View) -->
                <div
                  v-if="expandedSubagentSessions[s.id] && subagentsByParentId[s.id]"
                  class="bg-purple-950/20 border-y border-purple-500/20 pl-8 sm:pl-12 pr-6 sm:pr-8 py-2 divide-y divide-purple-500/10 space-y-1.5"
                >
                  <div
                    v-for="sub in subagentsByParentId[s.id]"
                    :key="sub.id"
                    @click="openSessionDetail(sub.id)"
                    class="py-2 hover:bg-paper-subtle/50 rounded-lg px-3 -mx-3 cursor-pointer flex items-center justify-between gap-4 transition text-xs group"
                  >
                    <div class="min-w-0">
                      <div class="flex items-center gap-2 flex-wrap">
                        <span class="text-cool text-xs">↳</span>
                        <span class="text-[10px] px-1.5 py-0.5 rounded font-mono bg-purple-500/20 text-purple-300 border border-purple-500/30">
                          Subagent: {{ sub.agent_name || 'agent' }}
                        </span>
                        <span class="font-mono text-ink group-hover:text-cool transition truncate font-medium">
                          {{ sub.title }}
                        </span>
                        <span class="text-[10px] badge-subtle px-1.5 py-0.5 rounded font-mono">
                          {{ sub.id.substring(0, 8) }}
                        </span>
                      </div>
                      <div class="flex items-center gap-3 text-[11px] text-ink-dim mt-1 font-mono pl-4">
                        <span>{{ new Date(sub.last_modified).toLocaleString('vi-VN') }}</span>
                        <span>• {{ sub.step_count }} bước</span>
                        <span>• {{ sub.model_name }}</span>
                      </div>
                    </div>
                    <div class="flex items-center gap-3 font-mono text-xs flex-shrink-0">
                      <span class="text-ink-muted hidden sm:inline">{{ formatNum(sub.total_tokens) }}</span>
                      <span class="px-2 py-0.5 rounded text-[10px] font-bold" :class="sub.cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'">
                        {{ sub.cache_hit_rate.toFixed(1) }}%
                      </span>
                      <span class="text-hit font-bold">
                        {{ formatCurrencyUSD(sub.cost_usd) }}
                      </span>
                      <span class="text-cool group-hover:translate-x-1 transition-transform">
                        &rarr;
                      </span>
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>
      </template>
    </main>
  </div>
</template>
