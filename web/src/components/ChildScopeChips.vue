<script setup lang="ts">
// 适用孩子 chip 排：第一颗「所有孩子」，其后按 children 顺序每个孩子一颗（只显示昵称）。
// modelValue：'all' = 所有孩子；number[] = 指定模式（[] = 指定模式但零选中）。
// 从 'all' 点某个孩子进入指定模式且只选它；再点同一颗取消；再点「所有孩子」回到全部。
import type { ChildView, ItemScope } from '../types'

const props = withDefaults(
  defineProps<{
    children: ChildView[]
    modelValue: ItemScope
    label?: string
  }>(),
  { label: '适用孩子' },
)

const emit = defineEmits<{ (e: 'update:modelValue', value: ItemScope): void }>()

function pickAll(): void {
  emit('update:modelValue', 'all')
}

function toggle(id: number): void {
  const cur = props.modelValue
  if (cur === 'all') {
    emit('update:modelValue', [id])
    return
  }
  emit('update:modelValue', cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id])
}
</script>

<template>
  <div role="group" :aria-label="label" class="flex flex-wrap gap-2">
    <button
      type="button"
      class="btn rounded-full max-w-full max-lg:btn-lg"
      :class="
        modelValue === 'all'
          ? 'btn-primary border-primary/25 bg-primary/10 text-brand-ink hover:border-primary/25 hover:bg-primary/15 hover:text-brand-ink'
          : 'btn-soft'
      "
      :aria-pressed="modelValue === 'all'"
      @click="pickAll"
    >
      <span class="truncate">所有孩子</span>
    </button>
    <button
      v-for="c in children"
      :key="c.id"
      type="button"
      class="btn rounded-full max-w-full max-lg:btn-lg"
      :class="
        modelValue !== 'all' && modelValue.includes(c.id)
          ? 'btn-primary border-primary/25 bg-primary/10 text-brand-ink hover:border-primary/25 hover:bg-primary/15 hover:text-brand-ink'
          : 'btn-soft'
      "
      :aria-pressed="modelValue !== 'all' && modelValue.includes(c.id)"
      @click="toggle(c.id)"
    >
      <span class="truncate">{{ c.displayName }}</span>
    </button>
  </div>
</template>
