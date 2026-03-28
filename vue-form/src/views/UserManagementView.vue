<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

interface UserItem {
  id: number
  username: string
  role: 'admin' | 'user'
  createdAt: string
}

const users = ref<UserItem[]>([])
const loading = ref(true)
const error = ref('')
const success = ref('')
const passwordDraft = ref<Record<string, string>>({})
const passwordSaving = ref<Record<string, boolean>>({})
const router = useRouter()
const auth = useAuthStore()

onMounted(async () => {
  if (!auth.checked) {
    await auth.fetchMe()
  }
  await loadUsers()
})

async function logout() {
  await fetch('/api/logout', { method: 'POST' })
  auth.setUser(null)
  router.push('/login')
}

async function loadUsers() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const res = await fetch('/api/admin/users')
    if (!res.ok) throw new Error('加载用户失败')
    const payload = await res.json()
    users.value = payload.items ?? []

    const nextDraft: Record<string, string> = {}
    for (const u of users.value) {
      nextDraft[String(u.id)] = ''
    }
    passwordDraft.value = nextDraft
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function updateUserRole(item: UserItem, role: 'admin' | 'user') {
  success.value = ''
  if (isProtectedAdmin(item)) {
    error.value = 'admin用户角色不可修改'
    return
  }
  if (item.id === auth.user?.id && role !== 'admin') {
    error.value = '当前登录管理员不能将自己降级为普通用户'
    return
  }

  error.value = ''
  const oldRole = item.role
  item.role = role

  try {
    const res = await fetch('/api/admin/user-role', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ userId: item.id, role }),
    })
    if (!res.ok) {
      const payload = await res.json()
      throw new Error(payload.error || '更新角色失败')
    }
    success.value = `用户 ${item.username} 角色已更新`
  } catch (e: any) {
    item.role = oldRole
    error.value = e.message || '更新失败'
  }
}

async function updateUserPassword(item: UserItem) {
	if (!canEditPassword(item)) {
		error.value = 'admin密码仅允许admin账户本人修改'
		return
	}

  const key = String(item.id)
  const newPassword = (passwordDraft.value[key] ?? '').trim()
  if (newPassword === '') {
    return
  }

  success.value = ''
  error.value = ''
  if (newPassword.length < 6) {
    error.value = '新密码至少 6 位'
    return
  }

  passwordSaving.value[key] = true
  try {
    const res = await fetch('/api/admin/user-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ userId: item.id, newPassword }),
    })
    if (!res.ok) {
      const payload = await res.json()
      throw new Error(payload.error || '修改密码失败')
    }
    passwordDraft.value[key] = ''
    success.value = `用户 ${item.username} 密码已更新`
  } catch (e: any) {
    error.value = e.message || '修改密码失败'
  } finally {
    passwordSaving.value[key] = false
  }
}

function canSavePassword(userID: number): boolean {
  const key = String(userID)
  return (passwordDraft.value[key] ?? '').trim().length > 0
}

function canEditPassword(item: UserItem): boolean {
  if (!isProtectedAdmin(item)) {
    return true
  }
  return auth.user?.username.trim().toLowerCase() === 'admin'
}

function isProtectedAdmin(item: UserItem): boolean {
  return item.username.trim().toLowerCase() === 'admin'
}
</script>

<template>
  <div class="page">
    <header class="site-header">
      <h1>用户管理</h1>
      <div class="header-right">
        <span v-if="auth.user" class="user-badge">{{ auth.user.username }}</span>
        <button class="btn-logout" @click="logout">退出登录</button>
        <a href="/admin" @click.prevent="router.push('/admin')" class="link">返回管理后台</a>
      </div>
    </header>

    <main class="container">
      <div v-if="loading" class="state-msg">用户加载中…</div>
      <div v-else-if="error" class="state-msg error">{{ error }}</div>
      <div v-else>
        <div v-if="success" class="inline-msg success">{{ success }}</div>

        <div class="table-wrap user-table-wrap">
          <table>
            <thead>
              <tr>
                <th>用户ID</th>
                <th>用户名</th>
                <th>角色</th>
                <th>创建时间</th>
                <th>新密码</th>
                <th>密码保存</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="u in users" :key="u.id">
                <td>{{ u.id }}</td>
                <td><span class="username">{{ u.username }}</span></td>
                <td>
                  <select
                    class="role-select"
                    :value="u.role"
                    :disabled="isProtectedAdmin(u)"
                    :title="isProtectedAdmin(u) ? 'admin用户角色不可修改' : ''"
                    @change="updateUserRole(u, ($event.target as HTMLSelectElement).value as 'admin' | 'user')"
                  >
                    <option value="user">普通用户</option>
                    <option value="admin">管理员</option>
                  </select>
                </td>
                <td>{{ u.createdAt }}</td>
                <td>
                  <input
                    v-model="passwordDraft[String(u.id)]"
                    class="pass-input"
                    type="password"
                    placeholder="输入新密码"
                    :disabled="!canEditPassword(u)"
                    :title="!canEditPassword(u) ? 'admin密码仅允许admin账户本人修改' : ''"
                  />
                </td>
                <td>
                  <button
                    class="btn-pass"
                    :disabled="passwordSaving[String(u.id)] || !canSavePassword(u.id) || !canEditPassword(u)"
                    :title="!canEditPassword(u) ? 'admin密码仅允许admin账户本人修改' : ''"
                    @click="updateUserPassword(u)"
                  >
                    {{ passwordSaving[String(u.id)] ? '密码保存中…' : '密码保存' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page { min-height: 100vh; background: transparent; }

.site-header {
  background: linear-gradient(135deg, var(--admin-header-start) 0%, var(--admin-header-end) 100%);
  color: var(--admin-header-text);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.site-header h1 { margin: 0; font-size: 1.3rem; }
.header-right { display: flex; align-items: center; gap: .9rem; }
.user-badge {
  background: rgba(255,255,255,.18);
  padding: .3rem .7rem;
  border-radius: 20px;
  font-size: .85rem;
}
.btn-logout {
  background: transparent;
  border: 1px solid rgba(255,255,255,.35);
  color: #fff;
  padding: .35rem .8rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: .85rem;
}
.link { color: rgba(255,255,255,.78); text-decoration: none; font-size: .85rem; }

.container { max-width: 980px; margin: 2rem auto; padding: 0 1rem; }
.state-msg { text-align: center; color: #888; padding: 3rem 0; }
.state-msg.error { color: #e53e3e; }

.inline-msg {
  font-size: .84rem;
  color: var(--status-success);
  margin-bottom: .55rem;
}

.table-wrap {
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  border-radius: 12px;
  border: 1px solid var(--surface-card-border);
  overflow: hidden;
  box-shadow: var(--shadow-soft);
}

table { width: 100%; border-collapse: collapse; }
th {
  background: #f6f8fd;
  padding: .9rem 1rem;
  text-align: left;
  font-size: .85rem;
  color: #5f6880;
  border-bottom: 1px solid #e6ebf3;
}

td {
  padding: .85rem 1rem;
  border-bottom: 1px solid #edf1f7;
  font-size: .9rem;
  vertical-align: middle;
}

.username {
  font-weight: 600;
  color: #1f2a44;
}

.role-select,
.pass-input {
  height: 32px;
  border-radius: 8px;
  border: 1px solid #d8dff1;
  padding: 0 .6rem;
  background: #fff;
  color: #334155;
  outline: none;
}

.role-select:focus,
.pass-input:focus {
  border-color: var(--brand-600);
  box-shadow: 0 0 0 3px var(--focus-ring);
}

.role-select:disabled {
  background: #f3f4f6;
  color: #94a3b8;
  cursor: not-allowed;
}

.pass-input:disabled {
  background: #f3f4f6;
  color: #94a3b8;
  cursor: not-allowed;
}

.pass-input {
  width: 160px;
}

.btn-pass {
  min-width: 78px;
  height: 30px;
  border: none;
  border-radius: 8px;
  background: #eef3ff;
  color: var(--brand-600);
  font-size: .8rem;
  font-weight: 600;
  cursor: pointer;
}

.btn-pass:hover { background: #e0e8ff; }
.btn-pass:disabled { opacity: .6; cursor: not-allowed; }
</style>
