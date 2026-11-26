<script setup lang="ts">
import type { Prize } from '../types'
import BaseDialog from './ui/BaseDialog.vue'

defineProps<{ prize: Prize; balance: number }>()
const emit = defineEmits<{ (e: 'close'): void }>()
</script>

<template>
  <!-- 可关闭：Esc / 点遮罩 = 原版的 @click.self -->
  <BaseDialog @close="emit('close')">
    <div class="flex flex-col items-center gap-2 py-2 text-center">
      <p class="font-extrabold text-primary">🎉 恭喜抽中 🎉</p>
      <p class="text-7xl">{{ prize.emoji }}</p>
      <p class="text-2xl font-extrabold">{{ prize.name }}</p>
      <p class="text-sm text-base-content/60">当前积分：{{ balance }}</p>
      <button class="btn btn-primary btn-block mt-2 max-lg:btn-lg" @click="emit('close')">
        收下
      </button>
    </div>
  </BaseDialog>
</template>
