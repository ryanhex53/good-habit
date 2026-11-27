<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, errMsg } from '../../api'
import { useDialogs } from '../../composables/useDialogs'
import EmptyState from '../../components/ui/EmptyState.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import RowCard from '../../components/ui/RowCard.vue'
import type { ItemView } from '../../types'

const items = ref<ItemView[]>([])
const error = ref('')
const newTitle = ref('')
const newPoints = ref<number | null>(null)

const dialogs = useDialogs()

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
  const v = await dialogs.prompt({
    title: '修改分值',
    label: `把「${it.title}」的分值改为：`,
    value: String(it.points),
    type: 'number',
  })
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
  const title = await dialogs.prompt({ title: '修改标题', label: '新标题', value: it.title })
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
    <PageHeader title="积分项" />
    <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>

    <div class="surface p-5">
      <div class="flex flex-col gap-3">
        <h3 class="text-base font-semibold">添加积分项</h3>
        <!-- 手机上竖向堆叠（join-vertical），lg 起横向拼接 -->
        <div class="join w-full max-lg:join-vertical">
          <input
            v-model="newTitle"
            class="input join-item w-full text-base max-lg:input-lg lg:w-auto lg:min-w-36 lg:flex-1"
            placeholder="标题，如：托管课写完作业"
          />
          <input
            v-model.number="newPoints"
            type="number"
            class="input join-item w-full text-base max-lg:input-lg lg:w-28"
            placeholder="分值"
          />
          <button
            class="btn btn-primary join-item max-lg:btn-lg"
            :disabled="!newTitle || !newPoints"
            @click="create"
          >
            添加
          </button>
        </div>
        <p class="text-xs text-base-content/50">
          停用的积分项不再出现在登记页与孩子的得分指南里
        </p>
      </div>
    </div>

    <div class="mt-4 flex flex-col gap-3">
      <RowCard v-for="it in items" :key="it.id" :class="!it.active ? 'opacity-60' : ''">
        <p class="flex flex-wrap items-center gap-2">
          <span class="font-medium">{{ it.title }}</span>
          <em class="badge badge-soft not-italic">+{{ it.points }} 分</em>
        </p>
        <template #actions>
          <button class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="rename(it)">改名</button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="editPoints(it)">
            改分值
          </button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="toggle(it)">
            {{ it.active ? '停用' : '启用' }}
          </button>
        </template>
      </RowCard>
    </div>
    <EmptyState v-if="items.length === 0" text="还没有积分项" />
  </div>
</template>
