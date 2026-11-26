<script setup lang="ts">
// 记分板：橙金渐变 + 全页唯一的主数字（TickNumber 滚动进场）。
// loaded 为假时用同字号的透明占位撑住高度，避免先闪一下 0。
import TickNumber from './TickNumber.vue'

defineProps<{ balance: number; loaded: boolean }>()
</script>

<template>
  <section
    class="relative overflow-hidden rounded-2xl bg-linear-to-br from-accent to-primary text-primary-content shadow-hero"
    aria-label="我的积分"
  >
    <div class="px-6 py-6 lg:px-8 lg:py-8">
      <p class="text-sm font-medium text-primary-content/80">我的积分</p>
      <p class="mt-2 flex items-baseline gap-2">
        <TickNumber
          v-if="loaded"
          :value="balance"
          enter
          class="text-hero lg:text-hero-lg"
        />
        <span v-else class="text-hero lg:text-hero-lg opacity-0" aria-hidden="true">0</span>
        <span class="text-lg font-medium text-primary-content/75">分</span>
      </p>
      <p class="mt-3 text-sm leading-relaxed text-primary-content/85">
        完成好事就能加分，攒够了去转盘抽奖
      </p>
    </div>
  </section>
</template>
