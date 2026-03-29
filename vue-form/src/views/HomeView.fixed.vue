<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

interface FormItem {
  Name: string
  Title: string
  Description: string
  Category?: string
  Pinned?: boolean
  SortOrder?: number
  Priority?: 'high' | 'medium' | 'low'
  Status?: 'draft' | 'published' | 'archived'
  PublishAt?: string
  ExpireAt?: string
}

const forms = ref<FormItem[]>([])
const loading = ref(true)
const error = ref('')
const router = useRouter()
const auth = useAuthStore()

const CATEGORY_LABELS: Record<string, string> = {
  general: '通用表单',
  hr: '人力资源',
  marketing: '市场运营',
  survey: '调研问卷',
  project: '项目管理',
}

const groupedForms = computed(() => {
  const map = new Map<string, FormItem[]>()
  for (const form of forms.value) {
    const key = (form.Category ?? 'general').trim().toLowerCase() || 'general'
    const list = map.get(key) ?? []
    list.push(form)
    map.set(key, list)
  }

  return Array.from(map.entries()).map(([key, items]) => ({
    key,
    title: CATEGORY_LABELS[key] ?? key,
    items,
  }))
})

onMounted(async () => {
  try {
    if (!auth.checked) {
      await auth.fetchMe()
    }

    const res = await fetch('/api/forms')
    if (!res.ok) throw new Error('加载失败')
    forms.value = await res.json()
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
})

async function logout() {
  await fetch('/api/logout', { method: 'POST' })
  auth.setUser(null)
  router.push('/login')
}

function formatDeadline(raw?: string): string {
  const value = (raw ?? '').trim()
  if (!value) return '长期有效'

  if (value.includes('T')) {
    const d = new Date(value)
    if (!Number.isNaN(d.getTime())) {
      const y = d.getFullYear()
      const m = String(d.getMonth() + 1).padStart(2, '0')
      const day = String(d.getDate()).padStart(2, '0')
      return `${y}-${m}-${day}`
    }
  }

  const datePart = value.split(' ')[0]
  return datePart ?? value
}

function isExpiringToday(raw?: string): boolean {
  const value = (raw ?? '').trim()
  if (!value) return false

  let expireDate: Date

  if (value.includes('T')) {
    const d = new Date(value)
    if (!Number.isNaN(d.getTime())) {
      expireDate = d
    } else {
      return false
    }
  } else {
    const datePart = value.split(' ')[0] ?? ''
    const [y, m, d] = datePart.split('-').map(Number)
    if (!y || !m || !d) return false
    expireDate = new Date(y, m - 1, d, 23, 59, 59)
  }

  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const tomorrow = new Date(today.getTime() + 86400000)

  return expireDate <= tomorrow
}
</script>

<template>
  <div class="page">
    <header class="site-header">
      <h1>表单中心</h1>
      <nav>
        <a v-if="auth.user" href="/change-password" @click.prevent="router.push('/change-password')">修改密码</a>
        <a v-if="auth.user?.role === 'admin'" href="/admin" @click.prevent="router.push('/admin')">管理后台</a>
        <a v-if="auth.user" href="/my-submissions" @click.prevent="router.push('/my-submissions')">我的提交</a>
        <a v-if="!auth.user" href="/login" @click.prevent="router.push('/login')">登录</a>
        <a v-if="!auth.user" href="/register" @click.prevent="router.push('/register')">注册</a>
        <a v-if="auth.user" href="#" @click.prevent="logout">退出</a>
      </nav>
    </header>

    <main class="container">
      <div v-if="auth.user" class="user-banner">
        <span class="banner-title">当前登录用户</span>
        <span class="banner-user">{{ auth.user.username }}</span>
        <span class="banner-role">{{ auth.user.role === 'admin' ? '管理员' : '普通用户' }}</span>
      </div>

      <div v-if="loading" class="state-msg">加载中…</div>
      <div v-else-if="error" class="state-msg error">{{ error }}</div>
      <div v-else-if="forms.length === 0" class="state-msg">暂无可用表单</div>

      <div v-else class="category-list">
        <section v-for="group in groupedForms" :key="group.key" class="category-block">
          <h2 class="category-title">{{ group.title }}</h2>
          <div class="table-wrap">
            <table class="form-table">
              <thead>
                <tr>
                  <th>表单</th>
                  <th>说明</th>
                  <th class="th-deadline">截止</th>
                  <th class="th-action">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="form in group.items"
                  :key="form.Name"
                  class="table-row"
                  @click="router.push(`/forms/${form.Name}`)"
                >
                  <td>
                    <div class="cell-main">
                      <h3 class="cell-title">
                        <span>{{ form.Title }}</span>
                        <span v-if="form.Pinned" class="cell-pin">置顶</span>
                      </h3>
                    </div>
                  </td>
                  <td>
                    <p class="cell-desc">{{ form.Description }}</p>
                  </td>
                  <td class="td-deadline">
                    <span :class="{ 'cell-deadline': true, 'deadline-urgent': isExpiringToday(form.ExpireAt) }">
                      {{ formatDeadline(form.ExpireAt) }}
                    </span>
                  </td>
                  <td class="td-action">
                    <button type="button" class="btn" @click.stop="router.push(`/forms/${form.Name}`)">
                      <span>填写</span>
                      <span class="btn-arrow">→</span>
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  min-height: 100dvh;
  background: transparent;
}

.site-header {
  background: var(--surface-header);
  border-bottom: 1px solid var(--surface-header-border);
  backdrop-filter: blur(8px);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.site-header h1 {
  font-size: 1.4rem;
  margin: 0;
  color: #1a1a2e;
}

.site-header nav {
  display: flex;
  align-items: center;
  gap: .9rem;
}

.site-header nav a {
  color: var(--brand-600);
  text-decoration: none;
  font-size: .9rem;
}

.container {
  max-width: 900px;
  margin: 2rem auto;
  padding: 0 1rem;
}

.user-banner {
  display: inline-flex;
  align-items: center;
  gap: .55rem;
  min-height: 36px;
  background: linear-gradient(90deg, #edf3ff 0%, #eaf9f0 100%);
  border: 1px solid #dbe5f7;
  border-radius: 999px;
  padding: 0 .9rem;
  margin-bottom: .95rem;
}

.banner-title {
  color: #60708e;
  font-size: .8rem;
}

.banner-user {
  color: #22314f;
  font-weight: 700;
  font-size: .88rem;
}

.banner-role {
  background: #fff;
  border: 1px solid #d8dff1;
  color: #445a84;
  border-radius: 999px;
  padding: .05rem .5rem;
  font-size: .75rem;
}

.state-msg {
  text-align: center;
  color: #888;
  padding: 3rem 0;
}

.state-msg.error {
  color: #e53e3e;
}

.category-list {
  display: grid;
  gap: .9rem;
}

.category-block {
  display: grid;
  gap: .56rem;
}

.category-title {
  margin: 0;
  font-size: .95rem;
  color: #354263;
}

.table-wrap {
  border: 1px solid var(--surface-card-border);
  border-radius: 12px;
  overflow: auto;
  background: #fff;
}

.form-table {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
}

.form-table th {
  background: #f3f7ff;
  color: #5b6784;
  font-size: .74rem;
  font-weight: 700;
  letter-spacing: .03em;
  text-transform: uppercase;
  padding: .58rem .72rem;
  text-align: left;
  border-bottom: 1px solid #dbe4f5;
}

.th-deadline,
.th-action,
.td-deadline,
.td-action {
  text-align: center;
}

.form-table td {
  padding: .66rem .72rem;
  border-bottom: 1px solid #e8eefb;
  vertical-align: middle;
}

.form-table tbody tr:last-child td {
  border-bottom: none;
}

.form-table tbody tr:nth-child(odd) td {
  background: #ffffff;
}

.form-table tbody tr:nth-child(even) td {
  background: #f8fbff;
}

.form-table tbody tr:hover td {
  background: #eef5ff;
}

.form-table tbody tr:hover td:first-child {
  box-shadow: inset 3px 0 0 var(--brand-600);
}

.table-row {
  cursor: pointer;
}

.cell-main {
  min-width: 0;
}

.cell-title {
  margin: 0;
  font-size: .95rem;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.3;
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: .25rem;
}

.cell-pin {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 .45rem;
  border-radius: 999px;
  background: #fff;
  border: 1px solid #d8dff1;
  color: #445a84;
  font-size: .66rem;
}

.cell-desc {
  margin: 0;
  color: #66738c;
  font-size: .8rem;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.cell-deadline {
  font-size: .72rem;
  color: #5f6c89;
  background: #f5f8ff;
  border: 1px solid #e2e9f8;
  border-radius: 999px;
  padding: .14rem .46rem;
  display: inline-flex;
  align-items: center;
}

.deadline-urgent {
  color: #dc2626;
  background: #fee2e2;
  border-color: #fecaca;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: .35rem;
  background: var(--brand-600);
  color: #fff;
  padding: .34rem .7rem;
  border-radius: 8px;
  font-size: .76rem;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: transform .18s ease, box-shadow .18s ease, background-color .18s ease;
}

.btn-arrow {
  transition: transform .18s ease;
}

.table-row:hover .btn {
  background: var(--brand-700);
  transform: translateY(-1px);
  box-shadow: 0 8px 14px rgba(63, 88, 214, .28);
}

.table-row:hover .btn-arrow {
  transform: translateX(2px);
}

@media (max-width: 768px) {
  .site-header {
    padding: .9rem 1rem;
    flex-direction: column;
    align-items: flex-start;
    gap: .65rem;
  }

  .site-header h1 {
    font-size: 1.14rem;
  }

  .site-header nav {
    width: 100%;
    flex-wrap: wrap;
    gap: .45rem .8rem;
  }

  .container {
    margin: 1rem auto;
  }

  .user-banner {
    min-height: auto;
    flex-wrap: wrap;
    padding: .4rem .72rem;
    border-radius: 12px;
  }

  .table-wrap {
    border-radius: 10px;
  }

  .form-table {
    min-width: 640px;
  }

  .form-table th,
  .form-table td {
    padding: .58rem .6rem;
  }

  .cell-title {
    font-size: .9rem;
  }

  .cell-desc {
    font-size: .78rem;
  }
}

@media (max-width: 430px) {
  .site-header {
    padding: .78rem .78rem;
  }

  .site-header nav a {
    font-size: .82rem;
  }

  .container {
    padding: 0 .75rem;
  }

  .user-banner {
    gap: .35rem .48rem;
    margin-bottom: .72rem;
  }

  .banner-title,
  .banner-user,
  .banner-role {
    font-size: .74rem;
  }

  .form-table {
    min-width: 560px;
  }

  .cell-title {
    font-size: .86rem;
  }

  .cell-deadline {
    font-size: .68rem;
  }

  .btn {
    padding: .3rem .58rem;
    font-size: .7rem;
  }
}

@media (max-width: 390px) {
  .site-header h1 {
    font-size: 1.05rem;
  }

  .site-header nav {
    gap: .34rem .66rem;
  }

  .form-table {
    min-width: 540px;
  }

  .form-table th,
  .form-table td {
    padding: .52rem .52rem;
  }
}

@media (max-width: 375px) {
  .site-header nav a {
    font-size: .78rem;
  }

  .form-table {
    min-width: 520px;
  }
}
</style>
