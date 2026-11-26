<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// 原生 <dialog> 封装：挂载即 showModal、卸载即 close，
// 焦点锁定与 Esc 由浏览器免费提供；dismissible=false 拦截 Esc 与遮罩点击。
// 开关状态归父组件（用 v-if 挂载/卸载本组件）。
const props = withDefaults(defineProps<{ dismissible?: boolean }>(), { dismissible: true })
const emit = defineEmits<{ (e: 'close'): void }>()

const el = ref<HTMLDialogElement | null>(null)

function onCancel(e: Event): void {
  if (!props.dismissible) {
    e.preventDefault()
    return
  }
  emit('close')
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
  <dialog ref="el" class="modal modal-bottom sm:modal-middle" @cancel="onCancel">
    <div class="modal-box animate-pop">
      <slot />
    </div>
    <form v-if="dismissible" method="dialog" class="modal-backdrop" @click="onBackdrop">
      <button>close</button>
    </form>
  </dialog>
</template>
