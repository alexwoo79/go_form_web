<script setup lang="ts">
import { RouterView } from 'vue-router'
import { onMounted, ref } from 'vue'

type ThemeMode = 'calm' | 'vivid'

const theme = ref<ThemeMode>('calm')

function applyTheme(mode: ThemeMode) {
  theme.value = mode
  if (mode === 'vivid') {
    document.documentElement.setAttribute('data-theme', 'vivid')
  } else {
    document.documentElement.removeAttribute('data-theme')
  }
  localStorage.setItem('ui-theme', mode)
}

function toggleTheme() {
  applyTheme(theme.value === 'calm' ? 'vivid' : 'calm')
}

onMounted(() => {
  const saved = localStorage.getItem('ui-theme') as ThemeMode | null
  applyTheme(saved === 'vivid' ? 'vivid' : 'calm')
})
</script>

<template>
  <button class="theme-switch" @click="toggleTheme">
    {{ theme === 'calm' ? '浅色活力' : '浅色商务' }}
  </button>
  <div class="app-shell">
    <main class="app-content">
      <RouterView />
    </main>
    <footer class="global-footer">
      <span>公司：crccredc</span>
      <span class="dot">•</span>
      <span>作者：Alex</span>
      <span class="dot">•</span>
      <span>版本：v1.0</span>
    </footer>
  </div>
</template>


<style scoped>
.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.app-content {
  flex: 1;
}

.theme-switch {
  position: fixed;
  right: 14px;
  bottom: 14px;
  z-index: 60;
  border: 1px solid rgba(206, 214, 232, .92);
  background: rgba(255, 255, 255, .9);
  color: var(--text-700);
  border-radius: 999px;
  height: 36px;
  padding: 0 .95rem;
  font-size: .82rem;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 8px 20px rgba(77, 95, 164, .14);
  backdrop-filter: blur(6px);
  transition: transform .15s ease, box-shadow .2s ease;
}

.theme-switch:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 22px rgba(77, 95, 164, .18);
}

.global-footer {
  margin: .6rem auto 1rem;
  display: inline-flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: .45rem;
  min-height: 32px;
  padding: 0 .85rem;
  border-radius: 999px;
  border: 1px solid rgba(206, 214, 232, .9);
  background: rgba(255, 255, 255, .88);
  color: var(--text-muted);
  font-size: .78rem;
  line-height: 1;
  box-shadow: 0 6px 16px rgba(77, 95, 164, .12);
  backdrop-filter: blur(6px);
}

.dot {
  color: #c0c7d9;
}

@media (max-width: 768px) {
  .global-footer {
    margin: .4rem .8rem .8rem;
    border-radius: 12px;
    padding: .45rem .7rem;
    line-height: 1.35;
  }

  .dot {
    display: none;
  }

  .theme-switch {
    right: 10px;
    bottom: 10px;
    height: 34px;
    padding: 0 .8rem;
    font-size: .78rem;
  }
}
</style>
