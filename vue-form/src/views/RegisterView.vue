<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()

async function register() {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = '两次输入的密码不一致'
    return
  }

  loading.value = true
  try {
    const res = await fetch('/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value }),
    })

    const data = await res.json()
    if (!res.ok) {
      error.value = data.error || '注册失败'
      return
    }

    auth.setUser(data.user)
    router.push('/my-submissions')
  } catch {
    error.value = '网络错误，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="register-page">
    <div class="register-card">
      <h1>注册账号</h1>
      <p class="subtitle">注册后可查看自己提交过的表单记录</p>

      <form @submit.prevent="register">
        <div class="field">
          <label>用户名</label>
          <input v-model="username" type="text" placeholder="请输入用户名（3-32字符）" required />
        </div>
        <div class="field">
          <label>密码</label>
          <input v-model="password" type="password" placeholder="请输入密码（至少6位）" required />
        </div>
        <div class="field">
          <label>确认密码</label>
          <input v-model="confirmPassword" type="password" placeholder="请再次输入密码" required />
        </div>

        <p v-if="error" class="error-msg">{{ error }}</p>

        <button type="submit" :disabled="loading">{{ loading ? '注册中…' : '注册' }}</button>
      </form>

      <p class="back-link">
        已有账号？
        <a href="/login" @click.prevent="router.push('/login')">去登录</a>
      </p>
    </div>
  </div>
</template>

<style scoped>
.register-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.register-card {
  background: rgba(255, 255, 255, .9);
  border-radius: 16px;
  padding: 2.2rem 2rem;
  width: 100%;
  max-width: 420px;
  border: 1px solid rgba(221, 227, 241, .9);
  box-shadow: 0 14px 36px rgba(76, 92, 148, .16);
  backdrop-filter: blur(8px);
}

h1 { margin: 0 0 .3rem; font-size: 1.45rem; color: #1a1a2e; }
.subtitle { color: #7f8798; margin: 0 0 1.4rem; font-size: .9rem; }

.field { margin-bottom: 1rem; }
.field label { display: block; font-size: .85rem; color: #444; margin-bottom: .4rem; font-weight: 500; }
.field input {
  width: 100%;
  padding: .65rem .85rem;
  border: 1.5px solid #d9dfeb;
  border-radius: 8px;
  font-size: 1rem;
  box-sizing: border-box;
}
.field input:focus {
  outline: none;
  border-color: var(--brand-600);
  box-shadow: 0 0 0 3px var(--focus-ring);
}

.error-msg { color: var(--status-danger); font-size: .85rem; margin: -.2rem 0 .8rem; }

button {
  width: 100%;
  padding: .75rem;
  background: var(--brand-600);
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  cursor: pointer;
}
button:hover:not(:disabled) { background: var(--brand-700); }
button:disabled { opacity: .6; cursor: not-allowed; }

.back-link { text-align: center; margin-top: 1rem; font-size: .85rem; color: #68748f; }
.back-link a { color: var(--brand-600); text-decoration: none; }
</style>
