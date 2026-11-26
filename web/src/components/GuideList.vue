<script setup lang="ts">
// 得分指南：一张纸面 + 分隔线清单（不再是每行一个胶囊）。
import type { GuideItem } from '../types'

defineProps<{ items: GuideItem[] }>()
</script>

<template>
  <details class="surface collapse collapse-arrow" open>
    <summary class="collapse-title min-h-12 px-5 py-4 text-base font-semibold lg:px-6">
      🏅 得分指南<span
        v-if="items.length > 0"
        class="ml-2 text-sm font-normal text-base-content/50"
      >{{ items.length }} 项</span>
    </summary>
    <div class="collapse-content px-0 pt-0 pb-1">
      <!-- px 不改变块级元素自身的盒位置（探针/缩进按盒左缘量），与清单行同缩进要用外边距 -->
      <p v-if="items.length === 0" class="mx-5 text-sm text-base-content/60 lg:mx-6">
        问问爸爸妈妈都设置好了哪些积分项吧
      </p>
      <ul v-else class="stagger divided px-5 lg:px-6">
        <li
          v-for="it in items"
          :key="it.id"
          class="flex items-center justify-between gap-3 py-3"
        >
          <span class="min-w-0 truncate text-base">{{ it.title }}</span>
          <span class="shrink-0 text-base font-semibold tabular-nums text-brand-ink">
            +{{ it.points }}<span class="ml-1 text-sm font-medium">分</span>
          </span>
        </li>
      </ul>
    </div>
  </details>
</template>
