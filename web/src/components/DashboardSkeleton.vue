<script setup lang="ts">
import { serverState } from '../stores/server';
</script>

<template>
  <div class="space-y-6 animate-pulse">
    <!-- Connecting Status Banner -->
    <div class="p-3.5 px-4 rounded-xl bg-paper-card border border-rule flex items-center justify-between gap-3 text-xs">
      <div class="flex items-center gap-2.5 min-w-0">
        <span class="relative flex h-2.5 w-2.5 flex-shrink-0">
          <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-cool opacity-75"></span>
          <span class="relative inline-flex rounded-full h-2.5 w-2.5 bg-cool"></span>
        </span>
        <span class="text-ink font-mono font-medium truncate">
          Đang kết nối đến máy chủ <strong class="text-cool">{{ serverState.endpoint }}</strong>...
        </span>
      </div>
      <span class="text-[11px] text-ink-dim font-mono hidden sm:inline flex-shrink-0">
        Quét transcripts &amp; tổng hợp metrics
      </span>
    </div>

    <!-- 5 KPI Overview Cards Skeleton -->
    <div class="grid grid-cols-2 lg:grid-cols-5 gap-3.5 sm:gap-4">
      <div
        v-for="i in 5"
        :key="i"
        :class="[
          'bg-paper-card border border-rule rounded-2xl p-4 sm:p-5 relative overflow-hidden shadow',
          i === 5 ? 'col-span-2 lg:col-span-1' : ''
        ]"
      >
        <div class="h-3 w-20 bg-paper-subtle rounded mb-2.5"></div>
        <div class="h-7 w-28 sm:w-32 bg-paper-subtle rounded mb-2"></div>
        <div class="h-2.5 w-24 bg-paper-subtle rounded mt-2"></div>
      </div>
    </div>

    <!-- 2 Charts Section Skeleton -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Model Distribution Chart Skeleton -->
      <div class="bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col">
        <div class="h-4 w-36 bg-paper-subtle rounded mb-1.5"></div>
        <div class="h-3 w-48 bg-paper-subtle rounded mb-4"></div>
        <div class="relative h-64 w-full flex items-center justify-center">
          <!-- Circular Doughnut Skeleton Outline -->
          <div class="w-40 h-40 rounded-full border-8 border-paper-subtle/80 flex items-center justify-center">
            <div class="w-20 h-20 rounded-full bg-paper"></div>
          </div>
        </div>
      </div>

      <!-- Daily Timeline Chart Skeleton -->
      <div class="lg:col-span-2 bg-paper-card border border-rule rounded-2xl p-5 shadow flex flex-col">
        <div class="h-4 w-44 bg-paper-subtle rounded mb-1.5"></div>
        <div class="h-3 w-56 bg-paper-subtle rounded mb-4"></div>
        <div class="relative h-64 w-full flex items-end justify-between gap-2 pt-6 px-4">
          <!-- Pulsing vertical bars of varying heights mimicking daily metrics -->
          <div
            v-for="(h, idx) in [45, 75, 30, 90, 60, 85, 40, 70, 95, 55, 80, 65]"
            :key="idx"
            class="flex-1 bg-paper-subtle rounded-t"
            :style="{ height: `${h}%` }"
          ></div>
        </div>
      </div>
    </div>

    <!-- Sessions Explorer Section Skeleton -->
    <div class="bg-paper-card border border-rule rounded-2xl shadow overflow-hidden">
      <!-- Header Toolbar Skeleton -->
      <div class="p-5 border-b border-rule flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div class="h-5 w-48 bg-paper-subtle rounded mb-1.5"></div>
          <div class="h-3 w-64 bg-paper-subtle rounded"></div>
        </div>
        <div class="flex items-center gap-2.5">
          <div class="h-8 w-44 bg-paper-subtle rounded-lg"></div>
          <div class="h-8 w-28 bg-paper-subtle rounded-lg"></div>
          <div class="h-8 w-20 bg-paper-subtle rounded-lg"></div>
        </div>
      </div>

      <!-- Mock Project Rows Skeleton -->
      <div class="divide-y divide-rule">
        <div v-for="r in 4" :key="r" class="p-4 sm:p-5 flex items-center justify-between gap-4">
          <div class="flex items-center gap-3.5 min-w-0">
            <div class="w-9 h-9 rounded-xl bg-paper-subtle flex-shrink-0"></div>
            <div class="space-y-1.5 min-w-0">
              <div class="h-4 w-36 sm:w-48 bg-paper-subtle rounded"></div>
              <div class="h-2.5 w-24 sm:w-32 bg-paper-subtle rounded"></div>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <div class="h-6 w-16 bg-paper-subtle rounded-full hidden sm:block"></div>
            <div class="h-6 w-20 bg-paper-subtle rounded-full hidden sm:block"></div>
            <div class="h-4 w-4 bg-paper-subtle rounded"></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
