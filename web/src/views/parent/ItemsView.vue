<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import type { ItemView } from '../../types'

const items = ref<ItemView[]>([])
const error = ref('')
const newTitle = ref('')
const newPoints = ref<number | null>(null)

async function load() {
  items.value = (await api<{ items: ItemView[] }>('GET', '/api/reward-items')).items
}

async function create() {
  error.value = ''
  try {
    await api('POST', '/api/reward-items', {
      title: newTitle.value,
      points: newPoints.value,
    })
    newTitle.value = ''
    newPoints.value = null
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function toggle(it: ItemView) {
  try {
    await api('PATCH', `/api/reward-items/${it.id}`, { active: !it.active })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function editPoints(it: ItemView) {
  const v = prompt(`把「${it.title}」的分值改为：`, String(it.points))
  if (!v) return
  const n = Number(v)
  if (!Number.isInteger(n) || n < 1) {
    error.value = '分值须为正整数'
    return
  }
  try {
    await api('PATCH', `/api/reward-items/${it.id}`, { points: n })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function rename(it: ItemView) {
  const title = prompt('新标题：', it.title)
  if (!title || title.trim() === '') return
  try {
    await api('PATCH', `/api/reward-items/${it.id}`, { title: title.trim() })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2>积分项</h2>
    <p class="err">{{ error }}</p>

    <div class="card add">
      <div class="add-row">
        <input v-model="newTitle" placeholder="标题，如：托管课写完作业" />
        <input v-model.number="newPoints" type="number" placeholder="分值" style="max-width: 120px" />
        <button :disabled="!newTitle || !newPoints" @click="create">添加</button>
      </div>
      <p class="hint">停用的积分项不再出现在登记页与孩子的得分指南里</p>
    </div>

    <div class="list">
      <div v-for="it in items" :key="it.id" class="card row" :class="{ off: !it.active }">
        <span class="title">{{ it.title }}</span>
        <span class="pts">+{{ it.points }} 分</span>
        <span class="ops">
          <button class="ghost" @click="rename(it)">改名</button>
          <button class="ghost" @click="editPoints(it)">改分值</button>
          <button class="ghost" @click="toggle(it)">{{ it.active ? '停用' : '启用' }}</button>
        </span>
      </div>
      <p v-if="items.length === 0" class="empty">还没有积分项</p>
    </div>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 14px;
}
.add-row {
  display: flex;
  gap: 8px;
}
.hint {
  font-size: 13px;
  color: var(--c-sub);
  margin-top: 8px;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 14px;
}
.row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.row.off {
  opacity: 0.6;
}
.title {
  flex: 1;
  font-size: 15px;
  font-weight: 600;
}
.pts {
  color: var(--c-primary);
  font-weight: 800;
  white-space: nowrap;
}
.ops {
  display: flex;
  gap: 6px;
}
.ops .ghost {
  padding: 6px 10px;
  font-size: 13px;
}
.empty {
  color: var(--c-sub);
  padding: 20px 0;
}
</style>
