<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, errMsg } from '../api'
import { useSession } from '../session'
import { homeOf } from '../router'
import BaseIcon from '../components/ui/BaseIcon.vue'
import FormField from '../components/ui/FormField.vue'

const router = useRouter()
const { me, refresh } = useSession()

const username = ref('')
const password = ref('')
const familyName = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await api('POST', '/api/auth/register', {
      username: username.value,
      password: password.value,
      family_name: familyName.value || undefined,
    })
    await refresh()
    // 同 LoginView：push('/') 经重定向回到当前路由会被静默中止，直接跳角色首页
    router.push(me.value ? homeOf(me.value.role) : '/login')
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="relative grid min-h-dvh overflow-hidden place-items-center bg-base-200 p-4">
    <div
      class="pointer-events-none absolute -top-24 -right-24 size-96 rounded-full bg-primary/10 blur-3xl"
      aria-hidden="true"
    ></div>

    <div class="card relative w-full max-w-sm bg-base-100 shadow-overlay">
      <div class="card-body gap-4">
        <div class="text-center">
          <div class="flex items-center justify-center gap-2">
            <span
              class="inline-flex size-10 items-center justify-center rounded-xl bg-primary text-primary-content"
            >
              <BaseIcon name="star" class="size-6" />
            </span>
            <h1 class="text-2xl font-bold">注册</h1>
          </div>
          <p class="mt-1 text-sm text-base-content/60">创建你的家庭积分账户</p>
        </div>

        <form class="flex flex-col gap-3" @submit.prevent="submit">
          <FormField label="用户名" hint="3-32 位小写字母/数字/下划线" for-id="username">
            <input
              id="username"
              v-model="username"
              class="input w-full text-base max-lg:input-lg"
              autocomplete="username"
            />
          </FormField>
          <FormField label="密码" hint="至少 8 位" for-id="password">
            <input
              id="password"
              v-model="password"
              type="password"
              class="input w-full text-base max-lg:input-lg"
              autocomplete="new-password"
            />
          </FormField>
          <FormField label="家庭名" hint="可不填，默认「我们家」" for-id="family">
            <input
              id="family"
              v-model="familyName"
              placeholder="我们家"
              class="input w-full text-base max-lg:input-lg"
            />
          </FormField>
          <p class="min-h-5 text-sm text-error" role="alert">{{ error }}</p>
          <button type="submit" class="btn btn-primary btn-block max-lg:btn-lg" :disabled="busy">
            注册并进入
          </button>
        </form>

        <p class="text-center text-sm text-base-content/60">
          已有账号？<RouterLink to="/login" class="link text-brand-ink">去登录</RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>
