<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const form = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})
const error = ref('')
const success = ref('')
const loading = ref(false)

async function changePassword() {
  error.value = ''
  success.value = ''

  if (form.value.newPassword.length < 6) {
    error.value = '新密码至少 6 位'
    return
  }
  if (form.value.newPassword !== form.value.confirmPassword) {
    error.value = '两次输入的新密码不一致'
    return
  }

  loading.value = true
  try {
    const res = await fetch('/api/user/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        oldPassword: form.value.oldPassword,
        newPassword: form.value.newPassword,
      }),
    })

    const payload = await res.json()
    if (!res.ok) {
      error.value = payload.error || '修改密码失败'
      return
    }

    success.value = '密码修改成功'
    form.value.oldPassword = ''
    form.value.newPassword = ''
    form.value.confirmPassword = ''
  } catch {
    error.value = '网络错误，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page">
    <header class="site-header">
      <h1>修改密码</h1>
      <nav>
        <a href="/" @click.prevent="router.push('/')">返回首页</a>
      </nav>
    </header>

    <main class="container">
      <div class="card">
        <form class="form" @submit.prevent="changePassword">
          <label>
            当前密码
            <input v-model="form.oldPassword" type="password" required />
          </label>
          <label>
            新密码
            <input v-model="form.newPassword" type="password" required />
          </label>
          <label>
            确认新密码
            <input v-model="form.confirmPassword" type="password" required />
          </label>

          <p v-if="error" class="msg error">{{ error }}</p>
          <p v-if="success" class="msg success">{{ success }}</p>

          <button type="submit" :disabled="loading">{{ loading ? '提交中…' : '确认修改' }}</button>
        </form>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page { min-height: 100vh; }

.site-header {
  background: var(--surface-header);
  border-bottom: 1px solid var(--surface-header-border);
  backdrop-filter: blur(8px);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.site-header h1 { margin: 0; font-size: 1.2rem; color: #1d2742; }
.site-header nav a { color: var(--brand-600); text-decoration: none; font-size: .9rem; }

.container { max-width: 520px; margin: 2rem auto; padding: 0 1rem; }

.card {
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  border: 1px solid var(--surface-card-border);
  border-radius: 12px;
  box-shadow: var(--shadow-soft);
  padding: 1rem;
}

.form {
  display: grid;
  gap: .8rem;
}

.form label {
  display: grid;
  gap: .35rem;
  font-size: .86rem;
  color: #4b5568;
}

.form input {
  height: 38px;
  border-radius: 8px;
  border: 1px solid #d8dff1;
  padding: 0 .65rem;
  outline: none;
}

.form input:focus {
  border-color: var(--brand-600);
  box-shadow: 0 0 0 3px var(--focus-ring);
}

.form button {
  height: 38px;
  border: none;
  border-radius: 8px;
  background: var(--brand-600);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.form button:disabled {
  opacity: .6;
  cursor: not-allowed;
}

.msg { font-size: .85rem; }
.msg.error { color: var(--status-danger); }
.msg.success { color: var(--status-success); }
</style>
