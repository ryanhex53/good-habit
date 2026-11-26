<script setup lang="ts">
import type { GuideItem } from '../types'

defineProps<{ items: GuideItem[] }>()
</script>

<template>
  <details class="collapse collapse-arrow border border-base-300 bg-base-100 shadow-sm" open>
    <summary class="collapse-title font-bold">🏅 得分指南</summary>
    <div class="collapse-content">
      <p v-if="items.length === 0" class="text-sm text-base-content/60">
        问问爸爸妈妈都设置好了哪些积分项吧
      </p>
      <ul v-else class="flex flex-col gap-2">
        <li
          v-for="it in items"
          :key="it.id"
          class="flex items-center justify-between gap-3 rounded-lg bg-base-200 px-3 py-2.5"
        >
          <span class="min-w-0 truncate text-sm">{{ it.title }}</span>
          <span class="text-sm font-bold whitespace-nowrap text-brand-ink">
            +{{ it.points }} 分
          </span>
        </li>
      </ul>
    </div>
  </details>
</template>
