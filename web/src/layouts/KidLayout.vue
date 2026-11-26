<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useSession } from '../session'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ThemeToggle from '../components/ui/ThemeToggle.vue'

const router = useRouter()
const route = useRoute()
const { me, logout } = useSession()

const links = [
  { to: '/kid/home', label: '首页', icon: 'home' },
  { to: '/kid/wheel', label: '转盘', icon: 'wheel' },
  { to: '/kid/bills', label: '账单', icon: 'receipt' },
]

function isActive(to: string): boolean {
  return route.path === to
}

async function doLogout() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="min-h-dvh bg-base-200">
    <!-- 顶栏：昵称 + 主题切换 + 登出（触控目标 ≥44px） -->
    <header class="navbar min-h-14 bg-base-100 px-3 pt-[env(safe-area-inset-top)]">
      <div class="flex-1 truncate font-bold">{{ me?.displayName }}</div>
      <div class="flex items-center gap-1">
        <ThemeToggle />
        <button
          type="button"
          class="btn btn-ghost max-lg:btn-circle min-h-11 min-w-11 gap-2"
          aria-label="退出登录"
          @click="doLogout"
        >
          <BaseIcon name="log-out" class="size-5" />
          <span class="hidden lg:inline">退出</span>
        </button>
      </div>
    </header>

    <!-- 桌面页签（≥1024px） -->
    <nav class="hidden justify-center px-4 pt-4 lg:flex">
      <div class="tabs tabs-box">
        <RouterLink
          v-for="l in links"
          :key="l.to"
          :to="l.to"
          class="tab min-h-11 gap-2"
          :class="isActive(l.to) ? 'tab-active text-brand-ink' : ''"
          :aria-current="isActive(l.to) ? 'page' : undefined"
        >
          <BaseIcon :name="l.icon" class="size-4" />
          {{ l.label }}
        </RouterLink>
      </div>
    </nav>

    <main
      class="mx-auto w-full max-w-2xl px-4 pt-4 pb-[calc(env(safe-area-inset-bottom)+5.5rem)] lg:pb-16"
    >
      <RouterView />
    </main>

    <!-- 手机底部 Tab 栏（不用 daisyUI dock：小屏会隐藏文字标签） -->
    <nav
      class="fixed inset-x-0 bottom-0 z-10 flex border-t border-base-300 bg-base-100 pb-[env(safe-area-inset-bottom)] lg:hidden"
    >
      <RouterLink
        v-for="l in links"
        :key="l.to"
        :to="l.to"
        class="flex min-h-14 flex-1 flex-col items-center justify-center gap-0.5"
        :class="isActive(l.to) ? 'font-semibold text-brand-ink' : 'text-base-content/60'"
        :aria-current="isActive(l.to) ? 'page' : undefined"
      >
        <BaseIcon :name="l.icon" class="size-5" />
        <span class="text-xs">{{ l.label }}</span>
      </RouterLink>
    </nav>
  </div>
</template>
