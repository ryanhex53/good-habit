<script setup lang="ts">
import { ref } from 'vue'
import type { GuideItem } from '../types'

defineProps<{ items: GuideItem[] }>()

const open = ref(true)
</script>

<template>
  <div class="card guide">
    <button class="toggle" @click="open = !open">
      <span>🏅 得分指南</span>
      <span class="arrow">{{ open ? '▾' : '▸' }}</span>
    </button>
    <p v-if="items.length === 0" class="empty">问问爸爸妈妈都设置好了哪些积分项吧</p>
    <ul v-if="open" class="list">
      <li v-for="it in items" :key="it.id">
        <span class="title">{{ it.title }}</span>
        <span class="pts">+{{ it.points }} 分</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.toggle {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: transparent;
  color: var(--c-ink);
  font-weight: 700;
  font-size: 16px;
  padding: 2px 0;
}
.arrow {
  color: var(--c-sub);
}
.list {
  list-style: none;
  padding: 0;
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.list li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  background: #faf8f4;
  border-radius: 10px;
}
.title {
  font-size: 15px;
}
.pts {
  color: var(--c-primary);
  font-weight: 700;
  font-size: 14px;
  white-space: nowrap;
}
.empty {
  color: var(--c-sub);
  font-size: 14px;
  margin-top: 8px;
}
</style>
