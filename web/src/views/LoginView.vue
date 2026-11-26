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
const busy = ref(false)
const error = ref('')

async function submit() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await api('POST', '/api/auth/login', { username: username.value, password: password.value })
    await refresh()
    // 修复（用户实测报告）：push('/') 会经 '/'→'/login' 重定向回到当前路由，
    // 被 vue-router 判定为重复导航而静默中止，导致登录后停在登录页。
    // 直接跳角色首页。
    router.push(me.value ? homeOf(me.value.role) : '/')
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="relative grid min-h-dvh overflow-hidden place-items-center bg-base-200 p-4">
    <!-- 淡橙装饰（纯装饰） -->
    <div
      class="pointer-events-none absolute -top-24 -right-24 size-96 rounded-full bg-primary/10 blur-3xl"
      aria-hidden="true"
    ></div>

    <div class="card relative w-full max-w-sm bg-base-100 shadow-md">
      <div class="card-body gap-4">
        <div class="text-center">
          <div class="flex items-center justify-center gap-2">
            <span
              class="inline-flex size-10 items-center justify-center rounded-xl bg-primary text-primary-content"
            >
              <BaseIcon name="star" class="size-6" />
            </span>
            <h1 class="text-2xl font-extrabold">好习惯</h1>
          </div>
          <p class="mt-1 text-sm text-base-content/60">积分攒起来，转盘抽惊喜</p>
        </div>

        <form class="flex flex-col gap-3" @submit.prevent="submit">
          <FormField label="用户名" for-id="username">
            <input
              id="username"
              v-model="username"
              class="input w-full text-base max-lg:input-lg"
              autocomplete="username"
              autofocus
            />
          </FormField>
          <FormField label="密码" for-id="password">
            <input
              id="password"
              v-model="password"
              type="password"
              class="input w-full text-base max-lg:input-lg"
              autocomplete="current-password"
            />
          </FormField>
          <p class="min-h-5 text-sm text-error" role="alert">{{ error }}</p>
          <button type="submit" class="btn btn-primary btn-block max-lg:btn-lg" :disabled="busy">
            登录
          </button>
        </form>

        <p class="text-center text-sm text-base-content/60">
          还没有账号？<RouterLink to="/register" class="link link-primary">注册一个</RouterLink>
        </p>
      </div>
    </div>
  </div>
</template>
