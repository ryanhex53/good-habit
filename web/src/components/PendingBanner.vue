<script setup lang="ts">
import type { PendingDraw } from '../types'

defineProps<{ pending: PendingDraw }>()
</script>

<template>
  <RouterLink
    :to="`/kid/wheel?resume=${pending.uuid}`"
    class="alert animate-pulse items-center border-2 border-dashed border-warning bg-warning/10 py-3"
  >
    <span class="text-2xl">🎁</span>
    <span class="min-w-0 flex-1">
      <b>有一局奖品还没选！</b>
      <small class="block truncate text-xs text-base-content/60">
        {{ pending.choices.map((c) => c.emoji + c.name).join(' / ') }}
      </small>
    </span>
    <span class="shrink-0 font-bold text-brand-ink">去选 ›</span>
  </RouterLink>
</template>
