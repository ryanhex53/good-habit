<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import BaseDialog from './BaseDialog.vue'
import { useDialogs } from '../../composables/useDialogs'

// 常驻组件：state.prompt 非空时挂载 BaseDialog。
// 语义对齐原生 prompt()：Enter 提交、空值不给确定、取消/Esc/遮罩 → null。
const { state, resolvePrompt } = useDialogs()
const value = ref('')
const inputEl = ref<HTMLInputElement | null>(null)

watch(
  () => state.prompt,
  async (opts) => {
    if (opts) {
      value.value = opts.value ?? ''
      await nextTick()
      inputEl.value?.focus()
      inputEl.value?.select()
    }
  },
)

function submit(): void {
  if (value.value.trim() === '') return
  resolvePrompt(value.value.trim())
}
</script>

<template>
  <BaseDialog v-if="state.prompt" :dismissible="true" @close="resolvePrompt(null)">
    <form class="flex flex-col gap-3" @submit.prevent="submit">
      <h3 class="text-lg font-bold">{{ state.prompt.title }}</h3>
      <label v-if="state.prompt.label" class="text-sm text-base-content/70" :for="'prompt-input'">
        {{ state.prompt.label }}
      </label>
      <input
        id="prompt-input"
        ref="inputEl"
        v-model="value"
        class="input w-full text-base max-lg:input-lg"
        :type="state.prompt.type ?? 'text'"
        :placeholder="state.prompt.placeholder"
        autocomplete="off"
      />
      <div class="modal-action">
        <button type="button" class="btn btn-ghost max-lg:btn-lg" @click="resolvePrompt(null)">
          取消
        </button>
        <button type="submit" class="btn btn-primary max-lg:btn-lg" :disabled="!value.trim()">
          {{ state.prompt.confirmText ?? '确定' }}
        </button>
      </div>
    </form>
  </BaseDialog>
</template>
