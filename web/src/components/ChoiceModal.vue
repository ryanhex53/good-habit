<script setup lang="ts">
import type { Prize } from '../types'

defineProps<{ choices: Prize[] }>()
const emit = defineEmits<{ (e: 'pick', index: number): void }>()
</script>

<template>
  <div class="mask">
    <div class="modal">
      <p class="title">挑一个你最喜欢的 🎁</p>
      <p class="sub">保险抽抽了两次，选中的就是你的！</p>
      <div class="cards">
        <button v-for="(c, i) in choices" :key="c.id" class="choice" @click="emit('pick', i)">
          <span class="emoji">{{ c.emoji }}</span>
          <span class="name">{{ c.name }}</span>
          <span class="tag">{{ i === 0 ? '第一抽' : '第二抽' }}</span>
        </button>
      </div>
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
}
.modal {
  background: #fff;
  border-radius: 20px;
  padding: 26px 22px;
  width: min(420px, 92vw);
  text-align: center;
  animation: pop 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
}
@keyframes pop {
  from {
    transform: scale(0.7);
    opacity: 0;
  }
}
.title {
  font-size: 20px;
  font-weight: 800;
}
.sub {
  color: var(--c-sub);
  font-size: 13px;
  margin: 6px 0 18px;
}
.cards {
  display: flex;
  gap: 14px;
}
.choice {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 20px 10px;
  background: #fff7e8;
  border: 2.5px solid #f0a24a;
  border-radius: 16px;
  color: var(--c-ink);
}
.choice:active {
  transform: scale(0.96);
}
.emoji {
  font-size: 44px;
}
.name {
  font-size: 16px;
  font-weight: 700;
}
.tag {
  font-size: 12px;
  color: var(--c-sub);
}
</style>
