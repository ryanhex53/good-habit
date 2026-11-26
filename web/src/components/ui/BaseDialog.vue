<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// 原生 <dialog> 封装：挂载即 showModal、卸载即 close，
// 焦点锁定与 Esc 由浏览器免费提供；dismissible=false 拦截 Esc 与遮罩点击。
// 开关状态归父组件（用 v-if 挂载/卸载本组件）。label 用作对话框的可访问名称。
const props = withDefaults(defineProps<{ dismissible?: boolean; label?: string }>(), {
  dismissible: true,
  label: undefined,
})
const emit = defineEmits<{ (e: 'close'): void }>()

const el = ref<HTMLDialogElement | null>(null)

function onCancel(e: Event): void {
  // 关闭一律经由父组件卸载完成；不可关闭的弹窗直接吞掉 Esc
  e.preventDefault()
  if (props.dismissible) emit('close')
}

function onBackdrop(): void {
  if (props.dismissible) emit('close')
}

onMounted(() => {
  el.value?.showModal()
})
onBeforeUnmount(() => {
  if (el.value?.open) el.value.close()
})
</script>

<template>
  <dialog
    ref="el"
    class="modal modal-bottom sm:modal-middle"
    :aria-label="label"
    @cancel="onCancel"
  >
    <div class="modal-box animate-pop">
      <slot />
    </div>
    <form v-if="dismissible" method="dialog" class="modal-backdrop" @click="onBackdrop">
      <button aria-label="关闭">关闭</button>
    </form>
  </dialog>
</template>
