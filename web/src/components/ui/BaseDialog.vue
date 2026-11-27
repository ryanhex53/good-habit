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

// 弹窗打开的时刻：物理双击时第二下（100–250ms 后）会落在刚出现的遮罩上，
// 开场 250ms 内忽略遮罩点击，避免确认框「开即关」、什么都没登记。
let openedAt = 0

function onCancel(e: Event): void {
  // 关闭一律经由父组件卸载完成；不可关闭的弹窗直接吞掉 Esc
  e.preventDefault()
  if (props.dismissible) emit('close')
}

function onBackdrop(e: Event): void {
  // 物理双击的第二下（100–250ms 后）会落在刚出现的遮罩上：开场 250ms 内整体忽略。
  // 注意 preventDefault 不能省——遮罩是 method="dialog" 的表单，只 return 的话
  // 浏览器仍会原生提交并关掉弹窗，且 confirm() 永远悬空。
  if (performance.now() - openedAt < 250) {
    e.preventDefault()
    return
  }
  if (props.dismissible) emit('close')
}

onMounted(() => {
  openedAt = performance.now()
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
    <div class="modal-box border-0 shadow-overlay animate-pop">
      <slot />
    </div>
    <form v-if="dismissible" method="dialog" class="modal-backdrop" @click="onBackdrop">
      <button aria-label="关闭">关闭</button>
    </form>
  </dialog>
</template>
