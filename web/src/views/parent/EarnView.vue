<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { api, errMsg } from '../../api'
import type { ChildView, EntryResp, ItemView } from '../../types'

const route = useRoute()

const children = ref<ChildView[]>([])
const items = ref<ItemView[]>([])
const activeChildren = computed(() => children.value.filter((c) => c.active))
const activeItems = computed(() => items.value.filter((i) => i.active))

const selected = ref<number | null>(null)
const note = ref('')
const toast = ref('')
const error = ref('')

// 手动调整
const adjDelta = ref<number | null>(null)
const adjNote = ref('')

function flash(msg: string) {
  toast.value = msg
  setTimeout(() => (toast.value = ''), 2200)
}

async function load() {
  children.value = (await api<{ children: ChildView[] }>('GET', '/api/children')).children
  items.value = (await api<{ items: ItemView[] }>('GET', '/api/reward-items')).items
  if (selected.value === null) {
    const fromQuery = Number(route.query.child)
    const found = activeChildren.value.find((c) => c.id === fromQuery)
    selected.value = found ? found.id : (activeChildren.value[0]?.id ?? null)
  }
}

async function earn(item: ItemView) {
  if (selected.value === null || toast.value) return
  error.value = ''
  try {
    const res = await api<EntryResp>('POST', '/api/earnings', {
      childId: selected.value,
      rewardItemId: item.id,
      note: note.value || undefined,
    })
    // 更新本地余额
    const c = children.value.find((x) => x.id === selected.value)
    if (c) c.balance = res.entry.balanceAfter
    flash(`已记 ${res.entry.delta > 0 ? '+' : ''}${res.entry.delta} 分（当前 ${res.entry.balanceAfter} 分）`)
    note.value = ''
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function adjust() {
  if (selected.value === null || adjDelta.value === null || adjDelta.value === 0) return
  error.value = ''
  try {
    const res = await api<EntryResp>('POST', '/api/adjustments', {
      childId: selected.value,
      delta: adjDelta.value,
      note: adjNote.value,
    })
    const c = children.value.find((x) => x.id === selected.value)
    if (c) c.balance = res.entry.balanceAfter
    flash(`已调整 ${res.entry.delta > 0 ? '+' : ''}${res.entry.delta} 分（当前 ${res.entry.balanceAfter} 分）`)
    adjDelta.value = null
    adjNote.value = ''
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2>登记得分</h2>

    <div class="card sel">
      <label>给谁登记</label>
      <div class="chips">
        <button
          v-for="c in activeChildren"
          :key="c.id"
          class="chip"
          :class="{ on: selected === c.id }"
          @click="selected = c.id"
        >
          {{ c.displayName }} · {{ c.balance }}分
        </button>
        <span v-if="activeChildren.length === 0" class="none">先去「孩子」页添加孩子</span>
      </div>
      <div class="field" style="margin-top: 12px">
        <label for="note">备注（可不填）</label>
        <input id="note" v-model="note" placeholder="今天完成得很好" />
      </div>
    </div>

    <p class="err">{{ error }}</p>
    <transition name="fade">
      <p v-if="toast" class="ok-note toast">{{ toast }}</p>
    </transition>

    <div class="grid">
      <button v-for="it in activeItems" :key="it.id" class="item" :disabled="selected === null" @click="earn(it)">
        <span class="pts">+{{ it.points }} 分</span>
        <span class="title">{{ it.title }}</span>
      </button>
    </div>
    <p v-if="activeItems.length === 0" class="empty">
      还没有积分项，去「积分项」页添加，或到总览页用一键导入
    </p>

    <div class="card adjust">
      <h3>手动调整</h3>
      <div class="adj-row">
        <input
          v-model.number="adjDelta"
          type="number"
          placeholder="±分值（如 -20 或 10）"
        />
        <input v-model="adjNote" placeholder="原因（必填）" />
        <button :disabled="selected === null || !adjDelta || !adjNote" @click="adjust">调整</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 14px;
}
.sel .chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.chip {
  background: #f4f1ea;
  color: var(--c-ink);
  padding: 8px 14px;
  border-radius: 20px;
  font-size: 14px;
}
.chip.on {
  background: var(--c-primary);
  color: #fff;
  font-weight: 700;
}
.none {
  color: var(--c-sub);
  font-size: 14px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 10px;
  margin: 14px 0;
}
.item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 10px;
  background: var(--c-card);
  color: var(--c-ink);
  box-shadow: 0 2px 8px rgba(43, 39, 34, 0.06);
}
.item:active {
  transform: scale(0.97);
}
.pts {
  color: var(--c-primary);
  font-weight: 800;
  font-size: 17px;
}
.title {
  font-size: 14px;
}
.toast {
  padding: 8px 0;
}
.empty {
  color: var(--c-sub);
  padding: 20px 0;
}
.adjust h3 {
  font-size: 16px;
  margin-bottom: 10px;
}
.adj-row {
  display: flex;
  gap: 8px;
}
.adj-row input {
  flex: 1;
}
.adj-row button {
  white-space: nowrap;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
