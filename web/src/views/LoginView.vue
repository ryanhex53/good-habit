<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, errMsg } from '../api'
import { useSession } from '../session'

const router = useRouter()
const { refresh } = useSession()

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
    router.push('/')
  } catch (e) {
    error.value = errMsg(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="auth-wrap">
    <div class="card auth-card">
      <h1>好习惯</h1>
      <p class="tagline">积分攒起来，转盘抽惊喜</p>
      <form @submit.prevent="submit">
        <div class="field">
          <label for="username">用户名</label>
          <input id="username" v-model="username" autocomplete="username" autofocus />
        </div>
        <div class="field">
          <label for="password">密码</label>
          <input id="password" v-model="password" type="password" autocomplete="current-password" />
        </div>
        <p class="err">{{ error }}</p>
        <button type="submit" :disabled="busy" style="width: 100%">登录</button>
      </form>
      <p class="switch">
        还没有账号？
        <RouterLink to="/register">注册一个</RouterLink>
      </p>
    </div>
  </div>
</template>

<style scoped>
.auth-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}
.auth-card {
  width: min(380px, 100%);
}
h1 {
  text-align: center;
  font-size: 26px;
  margin-bottom: 4px;
}
.tagline {
  text-align: center;
  color: var(--c-sub);
  margin-bottom: 22px;
  font-size: 14px;
}
.switch {
  margin-top: 16px;
  text-align: center;
  font-size: 14px;
  color: var(--c-sub);
}
</style>
