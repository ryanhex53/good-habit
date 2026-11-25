<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, errMsg } from '../api'
import { useSession } from '../session'

const router = useRouter()
const { refresh } = useSession()

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
      <h1>注册</h1>
      <p class="tagline">创建你的家庭积分账户</p>
      <form @submit.prevent="submit">
        <div class="field">
          <label for="username">用户名（3-32 位小写字母/数字/下划线）</label>
          <input id="username" v-model="username" autocomplete="username" />
        </div>
        <div class="field">
          <label for="password">密码（至少 8 位）</label>
          <input id="password" v-model="password" type="password" autocomplete="new-password" />
        </div>
        <div class="field">
          <label for="family">家庭名（可不填，默认"我们家"）</label>
          <input id="family" v-model="familyName" placeholder="我们家" />
        </div>
        <p class="err">{{ error }}</p>
        <button type="submit" :disabled="busy" style="width: 100%">注册并进入</button>
      </form>
      <p class="switch">
        已有账号？
        <RouterLink to="/login">去登录</RouterLink>
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
