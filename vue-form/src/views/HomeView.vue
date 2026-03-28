<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

interface FormItem {
  Name: string
  Title: string
  Description: string
  ExpireAt?: string
}

const forms = ref<FormItem[]>([])
const loading = ref(true)
const error = ref('')
const router = useRouter()
const auth = useAuthStore()

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

  // RFC3339 场景：2026-12-31T23:59:59+08:00
  if (value.includes('T')) {
    const d = new Date(value)
    if (!Number.isNaN(d.getTime())) {
      const y = d.getFullYear()
      const m = String(d.getMonth() + 1).padStart(2, '0')
      const day = String(d.getDate()).padStart(2, '0')
      return `${y}-${m}-${day}`
    }
  }

  // 兼容 2026-12-31 23:59:59 / 2026-12-31
  const datePart = value.split(' ')[0]
  return datePart ?? value
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

      <div v-else class="form-grid">
        <div
          v-for="form in forms"
          :key="form.Name"
          class="form-card"
          @click="router.push(`/forms/${form.Name}`)"
        >
          <span class="card-kicker">在线表单</span>
          <h2 class="card-title">{{ form.Title }}</h2>
          <p class="card-desc">{{ form.Description }}</p>
          <p class="card-deadline">填写截止：{{ formatDeadline(form.ExpireAt) }}</p>
          <div class="card-actions">
            <span class="btn">
              <span>填写</span>
              <span class="btn-arrow">→</span>
            </span>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page { min-height: 100vh; background: transparent; }

.site-header {
  background: var(--surface-header);
  border-bottom: 1px solid var(--surface-header-border);
  backdrop-filter: blur(8px);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.site-header h1 { font-size: 1.4rem; margin: 0; color: #1a1a2e; }
.site-header nav a {
  color: var(--brand-600);
  text-decoration: none;
  font-size: 0.9rem;
}

.site-header nav {
  display: flex;
  align-items: center;
  gap: .9rem;
}

.container { max-width: 900px; margin: 2rem auto; padding: 0 1rem; }

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

.state-msg { text-align: center; color: #888; padding: 3rem 0; }
.state-msg.error { color: #e53e3e; }

.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 1.2rem;
}

.form-card {
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  border-radius: 14px;
  padding: 1.5rem;
  cursor: pointer;
  border: 1px solid var(--surface-card-border);
  min-height: 196px;
  display: flex;
  flex-direction: column;
  transition: box-shadow .22s ease, transform .22s ease, border-color .22s ease;
}
.form-card:hover {
  border-color: #d5def2;
  box-shadow: var(--shadow-soft);
  transform: translateY(-3px);
}

.card-kicker {
  display: inline-flex;
  align-self: flex-start;
  padding: .2rem .55rem;
  border-radius: 999px;
  background: var(--bg-soft-violet);
  color: #5b4fd4;
  font-size: .72rem;
  font-weight: 600;
  letter-spacing: .02em;
  margin-bottom: .55rem;
}

.card-title {
  margin: 0;
  font-size: 1.08rem;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 2.9em;
}

.card-desc {
  margin: .45rem 0 0;
  color: #66738c;
  font-size: .9rem;
  line-height: 1.55;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 3.1em;
}

.card-deadline {
  margin: .55rem 0 0;
  font-size: .8rem;
  color: #5f6c89;
  background: #f5f8ff;
  border: 1px solid #e2e9f8;
  border-radius: 999px;
  align-self: flex-start;
  padding: .18rem .55rem;
}

.card-actions {
  margin-top: auto;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding-top: 1rem;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: .35rem;
  background: var(--brand-600);
  color: #fff;
  padding: .44rem .95rem;
  border-radius: 8px;
  font-size: .85rem;
  font-weight: 600;
  transition: transform .18s ease, box-shadow .18s ease, background-color .18s ease;
}

.btn-arrow {
  transition: transform .18s ease;
}

.form-card:hover .btn {
  background: var(--brand-700);
  transform: translateY(-1px);
  box-shadow: 0 8px 14px rgba(63, 88, 214, .28);
}

.form-card:hover .btn-arrow {
  transform: translateX(2px);
}
</style>

