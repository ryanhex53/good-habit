<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api } from '../../api'
import EmptyState from '../../components/ui/EmptyState.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import type { OverviewInfo } from '../../types'

const info = ref<OverviewInfo | null>(null)

const unfulfilledTotal = computed(() => (info.value?.children ?? []).reduce((n, c) => n + c.unfulfilled, 0))
const pendingCount = computed(() => (info.value?.children ?? []).filter((c) => c.hasPending).length)

onMounted(async () => {
  info.value = await api<OverviewInfo>('GET', '/api/overview')
})
</script>

<template>
  <div>
    <PageHeader title="总览" />

    <template v-if="info">
      <!-- 概况：三个统计共用一张纸面；有待办的两格用状态点标出 -->
      <section class="surface mt-5 grid grid-cols-3 divide-x divide-base-300/80" aria-label="概况">
        <div class="px-4 py-4 sm:px-5">
          <p class="text-xs text-base-content/60 sm:text-sm">今日得分</p>
          <p class="mt-1 flex items-baseline gap-1">
            <span class="text-stat font-semibold max-sm:text-2xl">{{ info.todayEarns }}</span>
            <span class="text-xs text-base-content/55 sm:text-sm">笔</span>
          </p>
        </div>
        <RouterLink to="/parent/draws" class="px-4 py-4 transition-colors hover:bg-base-200/60 sm:px-5">
          <p class="flex items-center gap-1.5 text-xs text-base-content/60 sm:text-sm">
            <span class="size-1.5 shrink-0 rounded-full" :class="unfulfilledTotal > 0 ? 'bg-warning' : 'bg-base-300'"></span>
            待发放奖品
          </p>
          <p class="mt-1 flex items-baseline gap-1">
            <span class="text-stat font-semibold max-sm:text-2xl">{{ unfulfilledTotal }}</span>
            <span class="text-xs text-base-content/55 sm:text-sm">个</span>
            <span class="ml-auto text-base-content/40" aria-hidden="true">›</span>
          </p>
        </RouterLink>
        <div class="px-4 py-4 sm:px-5">
          <p class="flex items-center gap-1.5 text-xs text-base-content/60 sm:text-sm">
            <span class="size-1.5 shrink-0 rounded-full" :class="pendingCount > 0 ? 'bg-accent' : 'bg-base-300'"></span>
            待选奖品
          </p>
          <p class="mt-1 flex items-baseline gap-1">
            <span class="text-stat font-semibold max-sm:text-2xl">{{ pendingCount }}</span>
            <span class="text-xs text-base-content/55 sm:text-sm">局</span>
          </p>
        </div>
      </section>

      <!-- 孩子：每人一张卡，余额是卡片的主角 -->
      <section class="mt-8">
        <h3 class="mb-3 text-base font-semibold">孩子</h3>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          <article v-for="c in info.children" :key="c.id" class="surface p-5">
            <div class="flex items-center justify-between gap-3">
              <h4 class="truncate text-base font-semibold">{{ c.displayName }}</h4>
              <span v-if="c.unfulfilled > 0 || c.hasPending" class="flex flex-wrap gap-1.5">
                <span v-if="c.unfulfilled > 0" class="status-chip">
                  <span class="size-1.5 rounded-full bg-warning"></span>{{ c.unfulfilled }} 个待发放
                </span>
                <span v-if="c.hasPending" class="status-chip">
                  <span class="size-1.5 rounded-full bg-accent"></span>待选奖品
                </span>
              </span>
            </div>
            <p class="mt-4 flex items-baseline gap-1.5">
              <span class="text-display">{{ c.balance }}</span>
              <span class="text-sm text-base-content/55">分</span>
            </p>
            <div class="mt-4">
              <RouterLink
                :to="`/parent/earn?child=${c.id}`"
                class="btn btn-soft btn-primary rounded-full text-brand-ink hover:text-brand-ink max-lg:btn-lg"
              >
                登记得分
              </RouterLink>
            </div>
          </article>
        </div>
      </section>

      <EmptyState
        v-if="info.children.length === 0"
        text="还没有孩子账号，先去「孩子」页添加一个"
      />
    </template>
  </div>
</template>
