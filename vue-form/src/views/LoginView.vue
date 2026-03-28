<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()

async function login() {
  error.value = ''
  loading.value = true
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value }),
    })
    if (res.ok) {
      // 从 /api/me 获取用户信息并同步到 store，再跳转
      await auth.fetchMe()
      if (auth.user?.role === 'admin') {
        router.push('/admin')
      } else {
        router.push('/my-submissions')
      }
    } else {
      const data = await res.json()
      error.value = data.error || '登录失败'
    }
  } catch {
    error.value = '网络错误，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <h1>用户登录</h1>
      <p class="subtitle">登录后可提交并查看自己的填报记录</p>

      <form @submit.prevent="login">
        <div class="field">
          <label>用户名</label>
          <input v-model="username" type="text" placeholder="请输入用户名" autocomplete="username" required />
        </div>
        <div class="field">
          <label>密码</label>
          <input v-model="password" type="password" placeholder="请输入密码" autocomplete="current-password" required />
        </div>
        <p v-if="error" class="error-msg">{{ error }}</p>
        <button type="submit" :disabled="loading">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </form>

      <p class="back-link">
        <a href="/" @click.prevent="router.push('/')">← 返回首页</a>
      </p>
      <p class="register-link">
        没有账号？
        <a href="/register" @click.prevent="router.push('/register')">去注册</a>
      </p>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  background:
    radial-gradient(circle at 16% 18%, #eaf1ff 0%, transparent 45%),
    radial-gradient(circle at 84% 16%, #f0fbf5 0%, transparent 40%),
    linear-gradient(145deg, #edf2ff 0%, #f4f6ff 45%, #f5f0ff 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
}

.login-card {
  background: rgba(255, 255, 255, .86);
  border-radius: 16px;
  padding: 2.5rem 2rem;
  width: 100%;
  max-width: 380px;
  border: 1px solid rgba(221, 227, 241, .9);
  box-shadow: 0 14px 36px rgba(76, 92, 148, .18);
  backdrop-filter: blur(8px);
}

h1 { margin: 0 0 .3rem; font-size: 1.5rem; color: #1a1a2e; }
.subtitle { color: #888; margin: 0 0 1.8rem; font-size: .9rem; }

.field { margin-bottom: 1.1rem; }
.field label { display: block; font-size: .85rem; color: #444; margin-bottom: .4rem; font-weight: 500; }
.field input {
  width: 100%;
  padding: .65rem .85rem;
  border: 1.5px solid #d9dfeb;
  border-radius: 8px;
  font-size: 1rem;
  box-sizing: border-box;
  transition: border-color .2s;
}
.field input:focus {
  outline: none;
  border-color: #4b68f2;
  box-shadow: 0 0 0 3px rgba(75, 104, 242, .13);
}

.error-msg { color: #e53e3e; font-size: .85rem; margin: -.3rem 0 .8rem; }

button {
  width: 100%;
  padding: .75rem;
  background: #4b68f2;
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  cursor: pointer;
  transition: background .2s;
}
button:hover:not(:disabled) { background: #3f58d6; }
button:disabled { opacity: .6; cursor: not-allowed; }

.back-link { text-align: center; margin-top: 1.2rem; font-size: .85rem; }
.back-link a { color: #4b68f2; text-decoration: none; }

.register-link {
  text-align: center;
  margin-top: .5rem;
  font-size: .82rem;
  color: #7a869f;
}

.register-link a {
  color: var(--brand-600);
  text-decoration: none;
}
</style>
