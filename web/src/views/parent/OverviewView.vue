<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import type { OverviewInfo } from '../../types'

const info = ref<OverviewInfo | null>(null)

onMounted(async () => {
  info.value = await api<OverviewInfo>('GET', '/api/overview')
})
</script>

<template>
  <div>
    <h2>总览</h2>
    <div class="stats">
      <div class="card stat">
        <p class="num">{{ info?.todayEarns ?? 0 }}</p>
        <p class="lbl">今日得分笔数</p>
      </div>
    </div>
    <div class="kids">
      <div v-for="c in info?.children ?? []" :key="c.id" class="card kid">
        <div class="row1">
          <span class="name">{{ c.displayName }}</span>
          <span class="badges">
            <em v-if="c.hasPending" class="b warn">待选奖品</em>
            <em v-if="c.unfulfilled > 0" class="b">{{ c.unfulfilled }} 个待发放</em>
          </span>
        </div>
        <p class="bal">
          <b>{{ c.balance }}</b> 分
        </p>
        <RouterLink class="go-earn" :to="`/parent/earn?child=${c.id}`">去登记得分 ›</RouterLink>
      </div>
      <p v-if="info && info.children.length === 0" class="empty">
        还没有孩子账号，先去「孩子」页添加一个
      </p>
    </div>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 14px;
}
.stats {
  display: flex;
  gap: 12px;
  margin-bottom: 14px;
}
.stat {
  flex: 1;
  text-align: center;
}
.num {
  font-size: 34px;
  font-weight: 800;
  color: var(--c-primary);
}
.lbl {
  font-size: 13px;
  color: var(--c-sub);
}
.kids {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 12px;
}
.kid .row1 {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}
.name {
  font-size: 17px;
  font-weight: 700;
}
.badges {
  display: flex;
  gap: 6px;
}
.b {
  font-style: normal;
  font-size: 12px;
  background: #fff2e4;
  color: var(--c-primary);
  padding: 2px 8px;
  border-radius: 20px;
}
.b.warn {
  background: #fff7e8;
  color: #b06a00;
}
.bal {
  font-size: 14px;
  color: var(--c-sub);
  margin: 8px 0 4px;
}
.bal b {
  font-size: 30px;
  color: var(--c-ink);
}
.go-earn {
  font-size: 13px;
  color: var(--c-primary);
  text-decoration: none;
}
.empty {
  color: var(--c-sub);
  padding: 30px 0;
}
</style>
