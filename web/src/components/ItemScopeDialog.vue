<script setup lang="ts">
// 「适用孩子」弹窗：草稿只含启用中的孩子，自己负责 PATCH、错误展示与「保存」禁用；
// 父组件（ItemsView）只负责挂载 / 卸载与刷新列表。
import { computed, ref } from 'vue'
import { api, errMsg } from '../api'
import BaseDialog from './ui/BaseDialog.vue'
import ChildScopeChips from './ChildScopeChips.vue'
import type { ChildView, ItemScope, ItemView } from '../types'

const props = defineProps<{ item: ItemView; children: ChildView[] }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

// 草稿只含启用中的孩子：把 childIds 过滤到启用中的 id。
// 孤儿项（被指定的孩子全部停用）→ 草稿为 []，即指定模式零选中。
const activeIds = new Set(props.children.map((c) => c.id))
const draft = ref<ItemScope>(
  props.item.childIds.length === 0 ? 'all' : props.item.childIds.filter((id) => activeIds.has(id)),
)
// 指定模式零选中时「保存」禁用（不得把空集当成「所有孩子」）
const canSave = computed(() => draft.value === 'all' || draft.value.length > 0)
const saving = ref(false)
const error = ref('')

async function save(): Promise<void> {
  if (!canSave.value || saving.value) return
  saving.value = true
  error.value = ''
  try {
    await api('PATCH', `/api/reward-items/${props.item.id}`, {
      childIds: draft.value === 'all' ? [] : draft.value, // 'all' 提交 [] = 所有孩子
    })
    emit('saved')
  } catch (e) {
    error.value = errMsg(e) // 失败不关弹窗，错误就地展示
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog :label="`适用孩子 · ${item.title}`" @close="!saving && emit('close')">
    <div class="flex flex-col gap-3">
      <h3 class="text-lg font-bold">适用孩子</h3>
      <p class="text-sm text-base-content/70">{{ item.title }}</p>
      <ChildScopeChips v-model="draft" :children="children" />
      <div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>
      <div class="modal-action">
        <button type="button" class="btn btn-ghost max-lg:btn-lg" @click="emit('close')">取消</button>
        <button
          type="button"
          class="btn btn-primary max-lg:btn-lg"
          :disabled="!canSave || saving"
          @click="save"
        >
          保存
        </button>
      </div>
    </div>
  </BaseDialog>
</template>
