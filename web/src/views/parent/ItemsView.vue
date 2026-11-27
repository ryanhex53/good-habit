<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, ApiError, errMsg } from '../../api'
import { useDialogs } from '../../composables/useDialogs'
import ChildScopeChips from '../../components/ChildScopeChips.vue'
import ItemScopeDialog from '../../components/ItemScopeDialog.vue'
import EmptyState from '../../components/ui/EmptyState.vue'
import PageHeader from '../../components/ui/PageHeader.vue'
import RowCard from '../../components/ui/RowCard.vue'
import type { ChildView, ItemScope, ItemView } from '../../types'

const children = ref<ChildView[]>([])
const items = ref<ItemView[]>([])
const error = ref('')
const newTitle = ref('')
const newPoints = ref<number | null>(null)
const newScope = ref<ItemScope>('all')
// 「适用孩子」弹窗的目标积分项；非空时挂载弹窗，关闭即置回 null
const scopeTarget = ref<ItemView | null>(null)

const activeChildren = computed(() => children.value.filter((c) => c.active))
// chip 排仅当启用中的孩子 ≥ 2 时显示（单孩家庭没有选择的意义）
const showAddChips = computed(() => activeChildren.value.length >= 2)
// 范围标签：chip 排可见，或列表里已有指定模式的项（停用孩子也可能被指定到）
const showScopeControls = computed(
  () => showAddChips.value || items.value.some((it) => it.childIds.length > 0),
)

const dialogs = useDialogs()

async function load() {
  const [kids, list] = await Promise.all([
    api<{ children: ChildView[] }>('GET', '/api/children'),
    api<{ items: ItemView[] }>('GET', '/api/reward-items'),
  ])
  children.value = kids.children
  items.value = list.items
}

async function create() {
  error.value = ''
  try {
    await api('POST', '/api/reward-items', {
      title: newTitle.value,
      points: newPoints.value,
      childIds: newScope.value === 'all' ? [] : newScope.value,
    })
    newTitle.value = ''
    newPoints.value = null
    newScope.value = 'all'
    load()
  } catch (e) {
    // 陈旧 newScope（勾选的孩子已在别处被停用）→ 回到所有孩子并刷新，别让「添加」卡死在 422
    if (e instanceof ApiError && e.code === 'child_not_found') {
      newScope.value = 'all'
      await load()
    }
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

// 「适用孩子」弹窗保存成功：关弹窗并刷新列表（范围标签可能变化）
function onScopeSaved(): void {
  scopeTarget.value = null
  load()
}

// 行内范围标签：无关联 = 所有孩子；指定模式用完整孩子列表（含停用）按 id 找名字，
// 已停用的加后缀「（已停用）」，找不到的 id 跳过。
function scopeLabel(it: ItemView): string {
  if (it.childIds.length === 0) return '所有孩子'
  const names: string[] = []
  for (const id of it.childIds) {
    const c = children.value.find((x) => x.id === id)
    if (!c) continue
    names.push(c.active ? c.displayName : `${c.displayName}（已停用）`)
  }
  return names.join('、')
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
            :disabled="!newTitle || !newPoints || (newScope !== 'all' && newScope.length === 0)"
            @click="create"
          >
            添加
          </button>
        </div>
        <ChildScopeChips v-if="showAddChips" v-model="newScope" :children="activeChildren" />
        <p class="text-xs text-base-content/50">
          停用的积分项不再出现在登记页与孩子的得分指南里
        </p>
      </div>
    </div>

    <div class="mt-6 flex flex-col gap-3">
      <RowCard v-for="it in items" :key="it.id" :class="!it.active ? 'opacity-60' : ''">
        <p class="flex flex-wrap items-center gap-2">
          <span class="font-medium">{{ it.title }}</span>
          <em class="badge badge-soft not-italic">+{{ it.points }} 分</em>
        </p>
        <p v-if="showScopeControls" class="scope-label truncate text-xs text-base-content/50">
          {{ scopeLabel(it) }}
        </p>
        <template #actions>
          <button class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="rename(it)">改名</button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="editPoints(it)">
            改分值
          </button>
          <button
            v-if="showScopeControls"
            class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm"
            @click="scopeTarget = it"
          >
            适用孩子
          </button>
          <button class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="toggle(it)">
            {{ it.active ? '停用' : '启用' }}
          </button>
        </template>
      </RowCard>
    </div>
    <EmptyState v-if="items.length === 0" text="还没有积分项" />
    <ItemScopeDialog
      v-if="scopeTarget"
      :item="scopeTarget"
      :children="activeChildren"
      @close="scopeTarget = null"
      @saved="onScopeSaved"
    />
  </div>
</template>
