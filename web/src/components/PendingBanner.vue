<script setup lang="ts">
import type { PendingDraw } from '../types'

defineProps<{ pending: PendingDraw }>()
</script>

<template>
  <RouterLink :to="`/kid/wheel?resume=${pending.uuid}`" class="banner">
    <span class="pulse">🎁</span>
    <span class="txt">
      <b>有一局奖品还没选！</b>
      <small>{{ pending.choices.map((c) => c.emoji + c.name).join(' / ') }}</small>
    </span>
    <span class="go">去选 ›</span>
  </RouterLink>
</template>

<style scoped>
.banner {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff7e8;
  border: 2px dashed #f0a24a;
  border-radius: 14px;
  padding: 12px 14px;
  text-decoration: none;
  color: var(--c-ink);
  animation: breathe 2.4s ease-in-out infinite;
}
@keyframes breathe {
  0%,
  100% {
    box-shadow: 0 0 0 0 rgba(240, 162, 74, 0.4);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(240, 162, 74, 0);
  }
}
.pulse {
  font-size: 26px;
}
.txt {
  flex: 1;
  display: flex;
  flex-direction: column;
  line-height: 1.4;
}
.txt small {
  color: var(--c-sub);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.go {
  font-weight: 700;
  color: var(--c-primary);
  white-space: nowrap;
}
</style>
