<script setup lang="ts">
// 待选奖品横幅：进页面时 🎁 晃两下提醒，之后保持安静（不再持续闪烁）。
import type { PendingDraw } from '../types'

defineProps<{ pending: PendingDraw }>()
</script>

<template>
  <RouterLink
    :to="`/kid/wheel?resume=${pending.uuid}`"
    class="flex items-center gap-3 rounded-2xl border-2 border-dashed border-warning bg-warning/10 py-3 px-4"
  >
    <span class="inline-block animate-wiggle text-2xl">🎁</span>
    <span class="min-w-0 flex-1">
      <b class="block text-base font-semibold">有一局奖品还没选！</b>
      <small class="block truncate text-sm text-base-content/60">
        {{ pending.choices.map((c) => c.emoji + c.name).join(' / ') }}
      </small>
    </span>
    <span class="shrink-0 text-sm font-semibold text-brand-ink">去选 ›</span>
  </RouterLink>
</template>
