<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import EmptyState from '../../components/ui/EmptyState.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import type { OverviewInfo } from '../../types'

const info = ref<OverviewInfo | null>(null)

onMounted(async () => {
  info.value = await api<OverviewInfo>('GET', '/api/overview')
})
</script>

<template>
  <div>
    <PageHeader title="总览" />

    <div class="stats w-full border border-base-300 bg-base-100 shadow-sm">
      <div class="stat">
        <div class="stat-value text-primary text-4xl tabular-nums">{{ info?.todayEarns ?? 0 }}</div>
        <div class="stat-title">今日得分笔数</div>
      </div>
    </div>

    <div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
      <div
        v-for="c in info?.children ?? []"
        :key="c.id"
        class="card border border-base-300 bg-base-100 shadow-sm"
      >
        <div class="card-body gap-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="text-lg font-bold">{{ c.displayName }}</span>
            <span class="flex flex-wrap gap-1.5">
              <em v-if="c.hasPending" class="badge badge-warning badge-soft not-italic">
                待选奖品
              </em>
              <em v-if="c.unfulfilled > 0" class="badge badge-soft not-italic">
                {{ c.unfulfilled }} 个待发放
              </em>
            </span>
          </div>
          <p class="text-sm text-base-content/60">
            <b class="text-3xl font-extrabold tabular-nums text-base-content">{{ c.balance }}</b>
            分
          </p>
          <RouterLink class="link text-brand-ink text-sm" :to="`/parent/earn?child=${c.id}`">
            去登记得分 ›
          </RouterLink>
        </div>
      </div>
    </div>

    <EmptyState
      v-if="info && info.children.length === 0"
      text="还没有孩子账号，先去「孩子」页添加一个"
    />
  </div>
</template>
