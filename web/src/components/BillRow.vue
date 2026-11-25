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
</script>

<template>
  <div class="row" :class="row.kind">
    <span class="icon">{{ icon }}</span>
    <div class="mid">
      <p class="reason">{{ reason }}</p>
      <p class="time">{{ fmtTime(row.createdAt) }}</p>
    </div>
    <div class="right">
      <p class="delta">{{ row.delta > 0 ? '+' : '' }}{{ row.delta }}</p>
      <p class="after">当时 {{ row.balanceAfter }}</p>
    </div>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--c-card);
  border-radius: 12px;
  padding: 12px 14px;
}
.icon {
  font-size: 22px;
}
.mid {
  flex: 1;
  min-width: 0;
}
.reason {
  font-size: 15px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.time {
  font-size: 12px;
  color: var(--c-sub);
  margin-top: 2px;
}
.right {
  text-align: right;
}
.delta {
  font-weight: 800;
  font-size: 17px;
}
.earn .delta {
  color: var(--c-ok);
}
.draw .delta,
.adjust .delta {
  color: var(--c-primary);
}
.after {
  font-size: 12px;
  color: var(--c-sub);
  margin-top: 2px;
}
</style>
