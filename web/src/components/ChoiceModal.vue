<script setup lang="ts">
import type { Prize } from '../types'
import BaseDialog from './ui/BaseDialog.vue'

// error：选择失败时的提示，渲染在弹窗内（弹窗处于顶层，页面内的行内提示会被遮住）
defineProps<{ choices: Prize[]; error?: string }>()
const emit = defineEmits<{ (e: 'pick', index: number): void }>()
</script>

<template>
  <!-- 必须选一个：Esc 与遮罩关闭均被拦截（对齐原版无关闭途径的行为） -->
  <BaseDialog :dismissible="false" label="选择奖品">
    <div class="flex flex-col gap-4">
      <div class="text-center">
        <h3 class="text-xl font-extrabold">挑一个你最喜欢的 🎁</h3>
        <p class="mt-1 text-sm text-base-content/60">保险抽抽了两次，选中的就是你的！</p>
      </div>
      <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>
      <div class="flex gap-3">
        <button
          v-for="(c, i) in choices"
          :key="c.id"
          class="btn h-auto flex-1 flex-col items-center justify-center gap-1.5 border-2 border-warning bg-warning/10 py-5 max-lg:btn-lg"
          @click="emit('pick', i)"
        >
          <span class="text-4xl">{{ c.emoji }}</span>
          <span class="font-bold">{{ c.name }}</span>
          <span class="text-xs text-base-content/60">{{ i === 0 ? '第一抽' : '第二抽' }}</span>
        </button>
      </div>
    </div>
  </BaseDialog>
</template>
