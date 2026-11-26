import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useSession } from './session'

const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/login' },
  { path: '/login', component: () => import('./views/LoginView.vue') },
  { path: '/register', component: () => import('./views/RegisterView.vue') },
  {
    path: '/kid',
    component: () => import('./layouts/KidLayout.vue'),
    meta: { role: 'child' },
    children: [
      { path: '', redirect: '/kid/home' },
      { path: 'home', component: () => import('./views/kid/HomeView.vue') },
      { path: 'wheel', component: () => import('./views/kid/WheelView.vue') },
      { path: 'bills', component: () => import('./views/kid/BillsView.vue') },
    ],
  },
  {
    path: '/parent',
    component: () => import('./layouts/ParentLayout.vue'),
    meta: { role: 'parent' },
    children: [
      { path: '', redirect: '/parent/overview' },
      { path: 'overview', component: () => import('./views/parent/OverviewView.vue') },
      { path: 'earn', component: () => import('./views/parent/EarnView.vue') },
      { path: 'children', component: () => import('./views/parent/ChildrenView.vue') },
      { path: 'items', component: () => import('./views/parent/ItemsView.vue') },
      { path: 'prizes', component: () => import('./views/parent/PrizesView.vue') },
      { path: 'draws', component: () => import('./views/parent/DrawsView.vue') },
      { path: 'bills', component: () => import('./views/parent/BillsView.vue') },
    ],
  },
]

const router = createRouter({ history: createWebHistory(), routes })

export function homeOf(role: 'parent' | 'child'): string {
  return role === 'parent' ? '/parent/overview' : '/kid/home'
}

router.beforeEach(async (to) => {
  const { me, refresh } = useSession()
  if (me.value === null) {
    await refresh()
  }

  // 登录/注册页：已登录则跳自己首页
  if (to.path === '/login' || to.path === '/register') {
    return me.value ? homeOf(me.value.role) : true
  }

  if (!me.value) {
    return '/login'
  }
  const need = to.matched.find((r) => r.meta.role)?.meta.role
  if (need && me.value.role !== need) {
    return homeOf(me.value.role)
  }
  return true
})

export default router
