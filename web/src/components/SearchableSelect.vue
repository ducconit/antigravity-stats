<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue';

export interface SelectOption {
  value: string;
  label: string;
  count?: number;
  subtitle?: string;
}

const props = withDefaults(
  defineProps<{
    modelValue: string;
    options: SelectOption[];
    placeholder?: string;
    searchPlaceholder?: string;
    icon?: 'folder' | 'sparkles' | 'sort' | 'filter' | 'none';
    clearable?: boolean;
    showAllOption?: boolean;
    allLabel?: string;
    align?: 'left' | 'right';
  }>(),
  {
    placeholder: 'Chọn...',
    searchPlaceholder: 'Tìm kiếm...',
    icon: 'none',
    clearable: true,
    showAllOption: true,
    allLabel: '',
    align: 'left',
  }
);

const emit = defineEmits<{
  (e: 'update:modelValue', val: string): void;
  (e: 'change', val: string): void;
}>();

const isOpen = ref(false);
const searchQuery = ref('');
const rootRef = ref<HTMLElement | null>(null);
const searchInputRef = ref<HTMLInputElement | null>(null);

const currentOption = computed(() => {
  return props.options.find((opt) => opt.value === props.modelValue);
});

const displayLabel = computed(() => {
  if (currentOption.value) {
    return currentOption.value.label;
  }
  return props.placeholder;
});

const isSelected = computed(() => Boolean(props.modelValue));

const effectiveAllLabel = computed(() => {
  return props.allLabel || props.placeholder || 'Tất cả';
});

const filteredOptions = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return props.options;
  return props.options.filter(
    (opt) =>
      opt.label.toLowerCase().includes(q) ||
      opt.value.toLowerCase().includes(q) ||
      (opt.subtitle && opt.subtitle.toLowerCase().includes(q))
  );
});

function toggleDropdown() {
  isOpen.value = !isOpen.value;
}

watch(isOpen, async (open) => {
  if (open) {
    searchQuery.value = '';
    await nextTick();
    searchInputRef.value?.focus();
  }
});

function selectOption(val: string) {
  emit('update:modelValue', val);
  emit('change', val);
  isOpen.value = false;
}

function handleClear(e: Event) {
  e.stopPropagation();
  emit('update:modelValue', '');
  emit('change', '');
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    isOpen.value = false;
  }
}

function handleClickOutside(e: PointerEvent) {
  if (rootRef.value && !rootRef.value.contains(e.target as Node)) {
    isOpen.value = false;
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener('pointerdown', handleClickOutside);
});
</script>

<template>
  <div ref="rootRef" class="relative inline-block text-left text-xs font-mono" @keydown="handleKeydown">
    <!-- Trigger Button -->
    <button
      type="button"
      @click="toggleDropdown"
      class="inline-flex items-center justify-between gap-2 px-3 py-1.5 rounded-lg border transition shadow-sm select-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-cool"
      :class="[
        isSelected
          ? 'bg-paper-subtle border-cool/40 text-ink hover:border-cool'
          : 'bg-paper border-rule text-ink-muted hover:text-ink hover:border-rule/80',
        isOpen ? 'border-cool ring-1 ring-cool' : '',
      ]"
      :title="displayLabel"
    >
      <!-- Leading Icon -->
      <span class="flex items-center gap-1.5 min-w-0">
        <!-- Folder Icon -->
        <svg
          v-if="icon === 'folder'"
          class="w-3.5 h-3.5 text-cool shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
        </svg>

        <!-- Sparkles / Model Icon -->
        <svg
          v-else-if="icon === 'sparkles'"
          class="w-3.5 h-3.5 text-accent shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M12 3v3m0 12v3M3 12h3m12 0h3M5.6 5.6l2.1 2.1m8.6 8.6l2.1 2.1M5.6 18.4l2.1-2.1m8.6-8.6l2.1-2.1" />
        </svg>

        <!-- Sort Icon -->
        <svg
          v-else-if="icon === 'sort'"
          class="w-3.5 h-3.5 text-hit shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M3 6h18M6 12h12M9 18h6" />
        </svg>

        <!-- Filter Icon -->
        <svg
          v-else-if="icon === 'filter'"
          class="w-3.5 h-3.5 text-cool shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
        </svg>

        <!-- Label -->
        <span class="truncate max-w-[130px] sm:max-w-[180px] lg:max-w-[220px]" :class="{ 'font-semibold text-ink': isSelected }">
          {{ displayLabel }}
        </span>
      </span>

      <!-- Count Badge or Status -->
      <span
        v-if="currentOption && currentOption.count !== undefined"
        class="text-[10px] px-1.5 py-0.2 rounded-full font-mono bg-paper border border-rule text-ink-dim shrink-0"
      >
        {{ currentOption.count }}
      </span>

      <!-- Clear & Chevron Group -->
      <span class="flex items-center gap-1 shrink-0 ml-1">
        <button
          v-if="clearable && isSelected"
          type="button"
          @click="handleClear"
          class="p-0.5 rounded text-ink-dim hover:text-ink hover:bg-paper cursor-pointer transition"
          title="Xóa lựa chọn"
        >
          <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>

        <svg
          class="w-3.5 h-3.5 text-ink-dim transition-transform duration-200"
          :class="{ 'rotate-180': isOpen }"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </span>
    </button>

    <!-- Dropdown Popover Menu -->
    <div
      v-if="isOpen"
      class="absolute z-50 mt-1 min-w-[240px] max-w-[calc(100vw-2rem)] sm:w-72 bg-paper-card border border-rule rounded-xl shadow-2xl overflow-hidden py-1.5 animate-in fade-in"
      :class="align === 'right' ? 'right-0' : 'left-0'"
    >
      <!-- Search Box inside dropdown -->
      <div class="px-2.5 pt-1 pb-2 border-b border-rule/70">
        <div class="relative">
          <svg
            class="w-3.5 h-3.5 text-ink-dim absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            type="text"
            :placeholder="searchPlaceholder"
            class="w-full pl-8 pr-7 py-1.5 rounded-lg bg-paper border border-rule text-xs text-ink placeholder-ink-dim font-mono focus:outline-none focus:border-cool focus:ring-1 focus:ring-cool transition"
          />
          <button
            v-if="searchQuery"
            type="button"
            @click="searchQuery = ''"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-ink-dim hover:text-ink p-0.5"
          >
            <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Options List -->
      <div class="max-h-60 overflow-y-auto divide-y divide-rule/30 py-1">
        <!-- "All" option if enabled -->
        <button
          v-if="showAllOption && !searchQuery"
          type="button"
          @click="selectOption('')"
          class="w-full text-left px-3 py-2 text-xs flex items-center justify-between gap-2 hover:bg-paper-subtle/80 transition cursor-pointer"
          :class="!modelValue ? 'text-cool font-bold bg-cool/10' : 'text-ink-muted hover:text-ink'"
        >
          <span class="truncate">{{ effectiveAllLabel }}</span>
          <span v-if="!modelValue" class="text-cool text-xs">✓</span>
        </button>

        <!-- No search results -->
        <div v-if="filteredOptions.length === 0" class="px-3 py-4 text-center text-xs text-ink-dim">
          Không tìm thấy kết quả phù hợp
        </div>

        <!-- Filtered items -->
        <button
          v-for="opt in filteredOptions"
          :key="opt.value"
          type="button"
          @click="selectOption(opt.value)"
          class="w-full text-left px-3 py-2 text-xs flex items-center justify-between gap-2 hover:bg-paper-subtle/80 transition cursor-pointer group"
          :class="modelValue === opt.value ? 'text-cool font-bold bg-cool/10' : 'text-ink-muted hover:text-ink'"
        >
          <div class="min-w-0 flex-1">
            <div class="truncate text-ink group-hover:text-cool transition">
              {{ opt.label }}
            </div>
            <div v-if="opt.subtitle" class="text-[10px] text-ink-dim truncate mt-0.5">
              {{ opt.subtitle }}
            </div>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <span
              v-if="opt.count !== undefined"
              class="text-[10px] px-1.5 py-0.5 rounded font-mono bg-paper border border-rule text-ink-dim"
            >
              {{ opt.count }}
            </span>
            <span v-if="modelValue === opt.value" class="text-cool text-xs">✓</span>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>
