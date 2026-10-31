<script setup lang="ts">
import { ref, watch } from 'vue';
import type { SessionMetric } from '../types';

const props = defineProps<{
  session: SessionMetric | null;
  isOpen: boolean;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const expandedThinking = ref<Record<number, boolean>>({});

watch(
  () => props.session,
  () => {
    expandedThinking.value = {};
  }
);

function toggleThinking(stepIndex: number) {
  expandedThinking.value[stepIndex] = !expandedThinking.value[stepIndex];
}

function formatNum(n?: number): string {
  if (n === undefined || n === null) return '0';
  return n.toLocaleString('vi-VN');
}

function formatCurrencyUSD(n?: number): string {
  if (n === undefined || n === null) return '$0.0000';
  return `$${n.toFixed(4)}`;
}

function formatCurrencyVND(n?: number): string {
  if (n === undefined || n === null) return '0 ₫';
  return `${Math.round(n).toLocaleString('vi-VN')} ₫`;
}
</script>

<template>
  <div
    v-if="isOpen && session"
    class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-black/80 backdrop-blur-sm animate-fade-in"
    @click.self="emit('close')"
  >
    <div
      class="bg-paper-card border border-rule rounded-2xl w-full max-w-5xl 2xl:max-w-7xl max-h-[92vh] flex flex-col shadow-2xl overflow-hidden"
    >
      <!-- Modal Header -->
      <div class="p-4 sm:p-5 border-b border-rule flex items-start justify-between gap-4 bg-paper/60 flex-shrink-0">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2 mb-1.5">
            <span class="px-2 py-0.5 rounded text-[11px] font-mono badge-cool font-semibold">
              {{ session.model_name || 'Gemini' }}
            </span>
            <span class="px-2 py-0.5 rounded text-[11px] font-medium badge-hit truncate max-w-xs">
              {{ session.project_name || 'Dự án' }}
            </span>
            <span class="text-xs text-ink-muted font-mono">
              ID: {{ session.id }}
            </span>
          </div>
          <h2 class="text-base sm:text-lg font-bold text-ink truncate leading-tight">
            {{ session.title || 'Hội thoại không có tiêu đề' }}
          </h2>
        </div>

        <button
          @click="emit('close')"
          class="p-2 rounded-lg bg-paper-subtle hover:bg-rule text-ink-muted hover:text-ink transition flex-shrink-0"
          title="Đóng (Esc)"
        >
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="18" y1="6" x2="6" y2="18"/>
            <line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        </button>
      </div>

      <!-- Quick Metrics Summary Bar -->
      <div class="px-4 sm:px-6 py-2.5 bg-paper-subtle border-b border-rule flex flex-wrap items-center justify-between gap-3 text-xs font-mono flex-shrink-0">
        <div class="flex items-center gap-3 sm:gap-4 flex-wrap">
          <span>Tổng số bước: <strong class="text-cool">{{ session.step_count }}</strong></span>
          <span>Input mới: <strong class="text-accent">{{ formatNum(session.input_tokens) }}</strong></span>
          <span>Cache Read: <strong class="text-hit">{{ formatNum(session.cache_read_tokens) }}</strong></span>
          <span>Output: <strong class="text-output">{{ formatNum(session.output_tokens) }}</strong></span>
        </div>
        <div class="flex items-center gap-2">
          <span
            class="px-2 py-0.5 rounded font-bold"
            :class="session.cache_hit_rate > 80 ? 'badge-hit' : 'badge-cool'"
          >
            Hit: {{ session.cache_hit_rate.toFixed(1) }}%
          </span>
          <span class="text-hit font-bold">
            {{ formatCurrencyUSD(session.cost_usd) }} ({{ formatCurrencyVND(session.cost_vnd) }})
          </span>
        </div>
      </div>

      <!-- Steps Waterfall List -->
      <div class="p-4 sm:p-6 overflow-y-auto space-y-3.5 flex-1">
        <div
          v-for="step in session.steps || []"
          :key="step.step_index"
          class="border rounded-xl p-3.5 sm:p-4 transition"
          :class="[
            step.source === 'USER_EXPLICIT'
              ? 'bg-paper-card border-cool-border'
              : (step.type === 'PLANNER_RESPONSE' ? 'bg-paper-card border-rule' : 'bg-paper/40 border-rule-subtle')
          ]"
        >
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-2">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-bold text-xs text-ink font-mono">
                Step {{ step.step_index }}
              </span>
              <span
                class="px-2 py-0.5 text-[10px] font-medium rounded-full"
                :class="step.source === 'USER_EXPLICIT' ? 'badge-cool' : 'badge-subtle'"
              >
                {{ step.source === 'USER_EXPLICIT' ? 'User Request' : (step.type === 'PLANNER_RESPONSE' ? 'Model Response' : step.type) }}
              </span>
              <span
                v-if="step.model_name"
                class="px-2 py-0.5 text-[10px] font-mono rounded badge-output"
              >
                {{ step.model_name }}
              </span>
              <span v-if="step.created_at" class="text-[10px] text-ink-dim font-mono">
                {{ new Date(step.created_at).toLocaleTimeString('vi-VN') }}
              </span>
            </div>

            <!-- Token Metrics for this Step -->
            <div
              v-if="step.type === 'PLANNER_RESPONSE'"
              class="flex flex-wrap items-center gap-2 sm:gap-3 font-mono text-[11px]"
            >
              <span class="text-accent" title="Input Tokens">In: {{ formatNum(step.input_tokens) }}</span>
              <span class="text-hit font-bold" title="Cache Read Tokens">
                Hit: {{ formatNum(step.cache_read_tokens) }} ({{ step.cache_hit_rate.toFixed(1) }}%)
              </span>
              <span class="text-output" title="Output Tokens">Out: {{ formatNum(step.output_tokens) }}</span>
              <span class="text-ink font-bold px-2 py-0.5 bg-paper border border-rule rounded">
                Total: {{ formatNum(step.total_step_tokens) }}
              </span>
            </div>
          </div>

          <!-- Tool Calls -->
          <div v-if="step.tool_calls && step.tool_calls.length > 0" class="mt-2 flex flex-wrap gap-1.5">
            <span
              v-for="(tool, tIdx) in step.tool_calls"
              :key="tIdx"
              class="px-2 py-1 rounded text-xs font-mono badge-cool flex items-center gap-1.5"
            >
              <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
              </svg>
              <strong>{{ tool.name }}</strong>
              <span v-if="tool.toolSummary" class="text-ink-muted text-[11px]">
                ({{ tool.toolSummary }})
              </span>
            </span>
          </div>

          <!-- Thinking CoT Preview -->
          <div v-if="step.thinking_preview" class="mt-2 text-xs">
            <button
              @click="toggleThinking(step.step_index)"
              class="text-ink-muted hover:text-cool flex items-center gap-1 font-mono transition"
            >
              <span>{{ expandedThinking[step.step_index] ? '▾ Thu gọn suy nghĩ' : '▸ Xem suy nghĩ (CoT Thinking)' }}</span>
            </button>
            <div
              v-if="expandedThinking[step.step_index]"
              class="mt-1.5 p-2.5 rounded bg-paper-subtle border border-rule text-ink-muted font-mono text-[11px] leading-relaxed whitespace-pre-wrap"
            >
              {{ step.thinking_preview }}
            </div>
          </div>

          <!-- Content Preview -->
          <div
            v-if="step.content_preview"
            class="mt-2 text-xs text-ink/90 font-mono bg-paper/60 p-2.5 rounded border border-rule-subtle line-clamp-3 hover:line-clamp-none transition"
          >
            {{ step.content_preview }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
