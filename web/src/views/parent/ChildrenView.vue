<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import { useDialogs } from '../../composables/useDialogs'
import { useToast } from '../../composables/useToast'
import RowCard from '../../components/ui/RowCard.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import type { ChildView } from '../../types'

const children = ref<ChildView[]>([])
const error = ref('')

const newName = ref('')
const newPw = ref('')
const newDisp = ref('')

const dialogs = useDialogs()
const toast = useToast()

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
    toast.show(`已创建 ${newDisp.value}`, 2200)
    newName.value = ''
    newPw.value = ''
    newDisp.value = ''
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function rename(c: ChildView) {
  const name = await dialogs.prompt({ title: '修改昵称', label: '新昵称', value: c.displayName })
  if (!name || name.trim() === '') return
  try {
    await api('PATCH', `/api/children/${c.id}`, { displayName: name.trim() })
    load()
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function resetPw(c: ChildView) {
  const pw = await dialogs.prompt({
    title: '重置密码',
    label: `给 ${c.displayName} 设新密码（至少 8 位）`,
    type: 'password',
    autocomplete: 'new-password',
  })
  if (!pw) return
  try {
    await api('PATCH', `/api/children/${c.id}`, { password: pw })
    toast.show(`${c.displayName} 的密码已重置`, 2200)
  } catch (e) {
    error.value = errMsg(e)
  }
}

async function toggleActive(c: ChildView) {
  const to = !c.active
  const verb = to ? '恢复' : '停用'
  const ok = await dialogs.confirm({
    title: `${verb} ${c.displayName}`,
    message: `确定${verb} ${c.displayName} 吗？${to === false ? '停用后其会话立即失效。' : ''}`,
    confirmText: verb,
    tone: to ? 'primary' : 'error',
  })
  if (!ok) return
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
    <PageHeader title="孩子管理" />
    <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>

    <div class="card border border-base-300 bg-base-100 shadow-sm">
      <div class="card-body gap-3">
        <h3 class="text-base font-bold">添加孩子</h3>
        <!-- 手机上每个字段独占一行、按钮整行；lg 起同一行 -->
        <div class="flex flex-col gap-2 lg:flex-row lg:flex-wrap">
          <input
            v-model="newDisp"
            class="input w-full text-base max-lg:input-lg lg:w-auto lg:min-w-36 lg:flex-1"
            placeholder="昵称（必填）"
          />
          <input
            v-model="newName"
            class="input w-full text-base max-lg:input-lg lg:w-auto lg:min-w-36 lg:flex-1"
            placeholder="登录用户名"
          />
          <input
            v-model="newPw"
            type="password"
            class="input w-full text-base max-lg:input-lg lg:w-auto lg:min-w-36 lg:flex-1"
            placeholder="初始密码（≥8 位）"
          />
          <button
            class="btn btn-primary max-lg:w-full max-lg:btn-lg"
            :disabled="!newDisp || !newName || !newPw"
            @click="create"
          >
            创建
          </button>
        </div>
        <p class="text-xs text-base-content/50">
          孩子用自己的用户名密码登录，只能看到自己的数据
        </p>
      </div>
    </div>

    <div class="mt-4 flex flex-col gap-3">
      <RowCard v-for="c in children" :key="c.id" :class="!c.active ? 'opacity-60' : ''">
        <p class="font-bold">
          {{ c.displayName }}
          <em v-if="!c.active" class="badge badge-ghost badge-sm ml-1 not-italic">已停用</em>
        </p>
        <p class="text-xs text-base-content/60">{{ c.username }} · {{ c.balance }} 分</p>
        <template #actions>
          <button class="btn btn-ghost max-lg:min-h-11 lg:btn-sm" @click="rename(c)">改名</button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:btn-sm" @click="resetPw(c)">
            重置密码
          </button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:btn-sm" @click="toggleActive(c)">
            {{ c.active ? '停用' : '恢复' }}
          </button>
        </template>
      </RowCard>
    </div>
  </div>
</template>
