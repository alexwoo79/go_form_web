import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import LoginView from '../views/LoginView.vue'
import AdminView from '../views/AdminView.vue'
import FormView from '../views/FormView.vue'
import RegisterView from '../views/RegisterView.vue'
import MySubmissionsView from '../views/MySubmissionsView.vue'
import ChangePasswordView from '../views/ChangePasswordView.vue'
import UserManagementView from '../views/UserManagementView.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: HomeView, meta: { requiresAuth: true, title: '表单中心' } },
    { path: '/login', name: 'login', component: LoginView, meta: { title: '用户登录' } },
    { path: '/register', name: 'register', component: RegisterView, meta: { title: '注册账号' } },
    {
      path: '/admin',
      name: 'admin',
      component: AdminView,
      meta: { requiresAuth: true, requiresAdmin: true, title: '管理后台' },
    },
    {
      path: '/admin/users',
      name: 'admin-users',
      component: UserManagementView,
      meta: { requiresAuth: true, requiresAdmin: true, title: '用户管理' },
    },
    {
      path: '/my-submissions',
      name: 'my-submissions',
      component: MySubmissionsView,
      meta: { requiresAuth: true, title: '我的提交' },
    },
    {
      path: '/change-password',
      name: 'change-password',
      component: ChangePasswordView,
      meta: { requiresAuth: true, title: '修改密码' },
    },
    {
      path: '/forms/:formName',
      name: 'form',
      component: FormView,
      meta: { requiresAuth: true, title: '填写表单' },
    },
    {
      path: '/s/:token',
      name: 'share-form',
      component: FormView,
      meta: { title: '填写表单' },
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()

  if (!auth.checked) {
    await auth.fetchMe()
  }

  if (!to.meta.requiresAuth) return

  if (!auth.isLoggedIn()) {
    return { name: 'login' }
  }

  if (to.meta.requiresAdmin && !auth.isAdmin()) {
    return { name: 'home' }
  }
})

router.afterEach((to) => {
  const baseTitle = '表单中心'
  const pageTitle = typeof to.meta.title === 'string' ? to.meta.title : baseTitle
  document.title = `${pageTitle} - ${baseTitle}`
})

export default router
