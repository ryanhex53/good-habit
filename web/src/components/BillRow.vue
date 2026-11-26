<script setup lang="ts">
import { computed } from 'vue'
import { fmtTime, type LedgerRow } from '../types'

const props = defineProps<{ row: LedgerRow }>()

const icon = computed(() =>
  props.row.kind === 'earn' ? '⭐' : props.row.kind === 'draw' ? '🎡' : '✏️',
)
const reason = computed(() => {
  if (props.row.kind === 'earn') return props.row.itemTitle
  if (props.row.kind === 'draw') {
    return `${props.row.prizeName}${props.row.mode === 'double' ? '（保险抽）' : ''}`
  }
  return props.row.note || '手动调整'
})
// 静态颜色映射（earn=绿，draw/adjust=品牌橙，与原实现一致；小字用 brand-ink 保证对比度）
const deltaClass: Record<string, string> = {
  earn: 'text-success',
  draw: 'text-brand-ink',
  adjust: 'text-brand-ink',
}
</script>

<template>
  <div class="surface">
    <div class="flex items-center gap-3 px-5 py-3.5">
      <span class="shrink-0 text-xl">{{ icon }}</span>
      <div class="min-w-0 flex-1">
        <p class="truncate text-base font-medium">{{ reason }}</p>
        <p class="text-xs text-base-content/60">{{ fmtTime(row.createdAt) }}</p>
      </div>
      <div class="shrink-0 text-right">
        <p class="text-lg font-semibold tabular-nums" :class="deltaClass[row.kind]">
          {{ row.delta > 0 ? '+' : '' }}{{ row.delta }}
        </p>
        <p class="text-xs text-base-content/60">当时 {{ row.balanceAfter }}</p>
      </div>
    </div>
  </div>
</template>
