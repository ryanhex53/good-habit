<script setup lang="ts">
import BaseDialog from './BaseDialog.vue'
import { useDialogs } from '../../composables/useDialogs'

// 常驻组件：state.confirm 非空时挂载。语义对齐原生 confirm()：
// 确定 → true；取消 / Esc / 遮罩 → false。
const { state, resolveConfirm } = useDialogs()
</script>

<template>
  <BaseDialog
    v-if="state.confirm"
    :label="state.confirm.title"
    :dismissible="true"
    @close="resolveConfirm(false)"
  >
    <div class="flex flex-col gap-3">
      <h3 class="text-lg font-bold">{{ state.confirm.title }}</h3>
      <p class="text-sm text-base-content/70">{{ state.confirm.message }}</p>
      <div class="modal-action">
        <button class="btn btn-ghost max-lg:btn-lg" @click="resolveConfirm(false)">
          {{ state.confirm.cancelText ?? '取消' }}
        </button>
        <button
          class="btn max-lg:btn-lg"
          :class="state.confirm.tone === 'error' ? 'btn-error' : 'btn-primary'"
          @click="resolveConfirm(true)"
        >
          {{ state.confirm.confirmText ?? '确定' }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>
