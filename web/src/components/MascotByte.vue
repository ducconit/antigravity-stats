<script setup lang="ts">
import { ref } from 'vue';

const robotSayings = [
  "Context Cache hoạt động bền bỉ! Não bộ AI chạy êm ru, sếp không lo cháy ví!",
  "Hơn 1.6 Tỷ token đã qua tay tôi! Viết đầy cả thư viện quốc gia luôn!",
  "1 turn chat tiết kiệm 30k token = bảo vệ thành công 1 cốc trà sữa cho dev!",
  "Gemini Flash & Claude đang miệt mài gõ code, còn tôi ngồi đây đếm token!",
  "Bíp bíp! Đừng cù tôi, tôi đang đếm token cho dự án của bạn đây!",
  "Lời khuyên kỹ thuật: Cache hit càng cao, ví tiền dev càng an toàn!",
  "Context Caching là phát minh vĩ đại nhất kể từ sau cà phê sáng!",
];

const sayingIndex = ref(0);
const isJumping = ref(false);

function poke() {
  isJumping.value = false;
  setTimeout(() => {
    isJumping.value = true;
    sayingIndex.value = (sayingIndex.value + 1) % robotSayings.length;
  }, 10);
  setTimeout(() => {
    isJumping.value = false;
  }, 700);
}
</script>

<template>
  <div class="w-full px-4 sm:px-6 lg:px-8 xl:px-10 2xl:px-12 pt-4">
    <div
      @click="poke"
      class="p-2.5 px-4 bg-paper-card border border-rule rounded-xl flex items-center justify-between gap-3 text-xs shadow-sm cursor-pointer hover:border-cool-border transition group"
      title="Bấm vào tôi để nghe thêm bí kíp Token!"
    >
      <div class="flex items-center gap-2.5 min-w-0">
        <!-- Cute Animated Mascot SVG -->
        <svg
          class="w-6 h-6 flex-shrink-0 text-cool transition-transform group-hover:scale-110"
          :class="{ 'mascot-jumping': isJumping }"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
        >
          <rect x="4" y="6" width="16" height="12" rx="3" stroke-width="2"/>
          <line x1="12" y1="2" x2="12" y2="6" stroke-width="2" stroke-linecap="round"/>
          <circle cx="12" cy="2" r="1" fill="currentColor"/>
          <circle cx="9" cy="11" r="1.5" fill="#10b981"/>
          <circle cx="15" cy="11" r="1.5" fill="#10b981"/>
          <path d="M9 15h6" stroke-width="1.8" stroke-linecap="round"/>
          <line x1="1" y1="12" x2="4" y2="12" stroke-width="2" stroke-linecap="round"/>
          <line x1="20" y1="12" x2="23" y2="12" stroke-width="2" stroke-linecap="round"/>
        </svg>

        <div class="min-w-0 flex items-center gap-2">
          <span class="font-bold text-cool flex-shrink-0">Byte:</span>
          <span class="text-ink-muted truncate transition-all duration-300">
            "{{ robotSayings[sayingIndex] }}"
          </span>
        </div>
      </div>

      <span class="text-[11px] text-ink-dim hidden sm:inline flex-shrink-0 group-hover:text-cool transition">
        Bấm đổi câu nói ▾
      </span>
    </div>
  </div>
</template>
