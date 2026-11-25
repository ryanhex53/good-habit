<script setup lang="ts">
import type { Prize } from '../types'

defineProps<{ prize: Prize; balance: number }>()
const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <div class="mask" @click.self="emit('close')">
    <div class="modal">
      <p class="cheer">🎉 恭喜抽中 🎉</p>
      <p class="emoji">{{ prize.emoji }}</p>
      <p class="name">{{ prize.name }}</p>
      <p class="bal">当前积分：{{ balance }}</p>
      <button @click="emit('close')">收下</button>
    </div>
  </div>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  background: rgba(30, 25, 20, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
  animation: fade 0.2s;
}
@keyframes fade {
  from {
    opacity: 0;
  }
}
.modal {
  background: #fff;
  border-radius: 20px;
  padding: 30px 36px;
  text-align: center;
  width: min(320px, 88vw);
  animation: pop 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
}
@keyframes pop {
  from {
    transform: scale(0.6);
    opacity: 0;
  }
}
.cheer {
  color: var(--c-primary);
  font-weight: 800;
}
.emoji {
  font-size: 64px;
  margin: 12px 0 4px;
}
.name {
  font-size: 22px;
  font-weight: 800;
  margin-bottom: 10px;
}
.bal {
  color: var(--c-sub);
  font-size: 14px;
  margin-bottom: 18px;
}
</style>
