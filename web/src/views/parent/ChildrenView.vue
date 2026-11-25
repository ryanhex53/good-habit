<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import type { ChildView } from '../../types'

const children = ref<ChildView[]>([])
const error = ref('')
const toast = ref('')

const newName = ref('')
const newPw = ref('')
const newDisp = ref('')

function flash(msg: string) {
  toast.value = msg
  setTimeout(() => (toast.value = ''), 2200)
}

async function load() {
  children.value = (await api<{ children: ChildView[] }>('GET', '/api/children')).children
}

async function create() {
  error.value = ''
  try {
    await api('POST', '/api/children', {
      username: newName.value,
      password: newPw.value,
      displayName: newDisp.value,
    })
    flash(`已创建 ${newDisp.value}`)
    newName.value = ''
    newPw.value = ''
    newDisp.value = ''
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function rename(c: ChildView) {
  const name = prompt('新昵称', c.displayName)
  if (!name || name.trim() === '') return
  try {
    await api('PATCH', `/api/children/${c.id}`, { displayName: name.trim() })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function resetPw(c: ChildView) {
  const pw = prompt(`给 ${c.displayName} 设新密码（至少 8 位）`)
  if (!pw) return
  try {
    await api('PATCH', `/api/children/${c.id}`, { password: pw })
    flash(`${c.displayName} 的密码已重置`)
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function toggleActive(c: ChildView) {
  const to = !c.active
  const verb = to ? '恢复' : '停用'
  if (!confirm(`确定${verb} ${c.displayName} 吗？${to === false ? '停用后其会话立即失效。' : ''}`)) return
  try {
    await api('PATCH', `/api/children/${c.id}`, { active: to })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2>孩子管理</h2>
    <p class="err">{{ error }}</p>
    <p v-if="toast" class="ok-note">{{ toast }}</p>

    <div class="card add">
      <h3>添加孩子</h3>
      <div class="add-row">
        <input v-model="newDisp" placeholder="昵称（必填）" />
        <input v-model="newName" placeholder="登录用户名" />
        <input v-model="newPw" type="password" placeholder="初始密码（≥8 位）" />
        <button :disabled="!newDisp || !newName || !newPw" @click="create">创建</button>
      </div>
      <p class="hint">孩子用自己的用户名密码登录，只能看到自己的数据</p>
    </div>

    <div class="list">
      <div v-for="c in children" :key="c.id" class="card row" :class="{ off: !c.active }">
        <div class="info">
          <p class="name">
            {{ c.displayName }}
            <em v-if="!c.active" class="tag">已停用</em>
          </p>
          <p class="sub">{{ c.username }} · {{ c.balance }} 分</p>
        </div>
        <div class="ops">
          <button class="ghost" @click="rename(c)">改名</button>
          <button class="ghost" @click="resetPw(c)">重置密码</button>
          <button class="ghost" @click="toggleActive(c)">{{ c.active ? '停用' : '恢复' }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
h2 {
  margin-bottom: 14px;
}
.add h3 {
  font-size: 16px;
  margin-bottom: 10px;
}
.add-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.add-row input {
  flex: 1;
  min-width: 140px;
}
.hint {
  font-size: 13px;
  color: var(--c-sub);
  margin-top: 8px;
}
.list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 14px;
}
.row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.row.off {
  opacity: 0.6;
}
.name {
  font-size: 17px;
  font-weight: 700;
}
.tag {
  font-style: normal;
  font-size: 12px;
  color: var(--c-warn);
  margin-left: 6px;
}
.sub {
  font-size: 13px;
  color: var(--c-sub);
}
.ops {
  display: flex;
  gap: 8px;
}
.ops .ghost {
  padding: 7px 12px;
  font-size: 13px;
}
</style>
