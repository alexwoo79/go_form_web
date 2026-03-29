<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

interface SubmissionItem {
  formName: string
  formTitle: string
  submittedAt: string
  ip?: string
  fields?: Array<{ Name: string; Label: string }>
  data: Record<string, unknown>
}

const items = ref<SubmissionItem[]>([])
const loading = ref(true)
const error = ref('')
const detailVisible = ref(false)
const currentItem = ref<SubmissionItem | null>(null)
const router = useRouter()

function previewData(data: Record<string, unknown>): string {
  const entries = Object.entries(data).slice(0, 3)
  if (entries.length === 0) return '-'
  return entries
    .map(([k, v]) => `${k}: ${Array.isArray(v) ? v.join(',') : String(v ?? '-')}`)
    .join(' | ')
}

function formatValue(value: unknown): string {
  if (Array.isArray(value)) return value.join(', ')
  if (value === null || value === undefined || value === '') return '-'
  return String(value)
}

function getDetailFields(item: SubmissionItem): Array<{ Name: string; Label: string }> {
  if (item.fields && item.fields.length > 0) {
    return item.fields
  }
  return Object.keys(item.data).map((k) => ({ Name: k, Label: k }))
}

function openDetail(item: SubmissionItem) {
  currentItem.value = item
  detailVisible.value = true
}

function closeDetail() {
  detailVisible.value = false
  currentItem.value = null
}

onMounted(async () => {
  try {
    const res = await fetch('/api/my/submissions')
    if (res.status === 401) {
      router.push('/login')
      return
    }
    if (!res.ok) throw new Error('加载失败')

    const payload = await res.json()
    items.value = payload.items ?? []
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page">
    <header class="site-header">
      <h1>我的提交</h1>
      <nav>
        <a href="/change-password" @click.prevent="router.push('/change-password')">修改密码</a>
        <a href="/" @click.prevent="router.push('/')">返回首页</a>
      </nav>
    </header>

    <main class="container">
      <div v-if="loading" class="state-msg">加载中…</div>
      <div v-else-if="error" class="state-msg error">{{ error }}</div>
      <div v-else-if="items.length === 0" class="state-msg">暂无提交记录</div>

      <div v-else class="list-wrap">
        <table>
          <thead>
            <tr>
              <th>表单</th>
              <th>提交时间</th>
              <th>内容预览</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(item, idx) in items" :key="`${item.formName}-${idx}`">
              <td>
                <div class="title">{{ item.formTitle }}</div>
                <div class="name">{{ item.formName }}</div>
              </td>
              <td>{{ item.submittedAt || '-' }}</td>
              <td class="preview">{{ previewData(item.data) }}</td>
              <td>
                <button class="btn-detail" @click="openDetail(item)">详情</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="detailVisible && currentItem" class="modal-mask" @click.self="closeDetail">
        <div class="modal-panel">
          <div class="modal-header">
            <h3>{{ currentItem.formTitle }} - 提交详情</h3>
            <button class="btn-close" @click="closeDetail">关闭</button>
          </div>
          <div class="detail-meta">
            <span>表单标识：{{ currentItem.formName }}</span>
            <span>提交时间：{{ currentItem.submittedAt || '-' }}</span>
          </div>
          <div class="detail-wrap">
            <table class="detail-table wide">
              <thead>
                <tr>
                  <th v-for="f in getDetailFields(currentItem)" :key="f.Name">{{ f.Label }}</th>
                  <th>提交时间</th>
                  <th>IP</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td v-for="f in getDetailFields(currentItem)" :key="`v-${f.Name}`">
                    {{ formatValue(currentItem.data[f.Name]) }}
                  </td>
                  <td>{{ currentItem.submittedAt || '-' }}</td>
                  <td>{{ currentItem.ip || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page {
  min-height: 100vh;
  min-height: 100dvh;
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
.site-header h1 { margin: 0; font-size: 1.2rem; color: #1d2742; }
.site-header nav a { color: var(--brand-600); text-decoration: none; font-size: .9rem; }
.site-header nav {
  display: flex;
  align-items: center;
  gap: .9rem;
}

.container { max-width: 980px; margin: 2rem auto; padding: 0 1rem; }
.state-msg { text-align: center; color: #8892a5; padding: 3rem 0; }
.state-msg.error { color: var(--status-danger); }

.list-wrap {
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  border: 1px solid var(--surface-card-border);
  border-radius: 12px;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  box-shadow: var(--shadow-soft);
}

table {
  width: 100%;
  min-width: 660px;
  border-collapse: collapse;
}
th {
  background: #f6f8fd;
  text-align: left;
  padding: .85rem 1rem;
  font-size: .84rem;
  color: #5f6880;
}
td {
  border-top: 1px solid #edf1f7;
  padding: .9rem 1rem;
  font-size: .9rem;
  color: #334155;
  vertical-align: top;
}
.title { font-weight: 600; color: #1d2742; }
.name { font-size: .8rem; color: #8a93a8; margin-top: .2rem; }
.preview { color: #5a6480; }

.btn-detail {
  min-width: 72px;
  height: 30px;
  border: none;
  border-radius: 8px;
  background: #eef3ff;
  color: var(--brand-600);
  font-size: .8rem;
  font-weight: 600;
  cursor: pointer;
}

.btn-detail:hover {
  background: #e0e8ff;
}

.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(21, 30, 53, .38);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  z-index: 50;
}

.modal-panel {
  width: min(880px, 96vw);
  max-height: 84vh;
  background: #fff;
  border-radius: 12px;
  border: 1px solid #e6ebf3;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: .9rem 1rem;
  border-bottom: 1px solid #edf1f7;
}

.modal-header h3 {
  margin: 0;
  font-size: 1rem;
  color: #1f2937;
}

.btn-close {
  border: 1px solid #d1d5db;
  background: #fff;
  color: #374151;
  border-radius: 6px;
  height: 32px;
  padding: 0 .8rem;
  cursor: pointer;
}

.detail-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  padding: .7rem 1rem;
  border-bottom: 1px solid #edf1f7;
  color: #5f6880;
  font-size: .85rem;
}

.detail-wrap {
  padding: .8rem 1rem 1rem;
  overflow: auto;
}

.detail-table {
  width: 100%;
  border-collapse: collapse;
}

.detail-table.wide {
  min-width: 700px;
}

.detail-table th,
.detail-table td {
  border-bottom: 1px solid #eef2f7;
  padding: .62rem .7rem;
  text-align: left;
  font-size: .85rem;
  vertical-align: top;
}

.detail-table th {
  background: #f6f8fd;
  color: #475569;
  font-weight: 600;
}

@media (max-width: 768px) {
  .site-header {
    padding: .9rem 1rem;
    flex-direction: column;
    align-items: flex-start;
    gap: .65rem;
  }

  .site-header h1 {
    font-size: 1.12rem;
  }

  .site-header nav {
    width: 100%;
    flex-wrap: wrap;
    gap: .45rem .8rem;
  }

  .container {
    margin: 1rem auto;
  }

  table {
    min-width: 620px;
  }

  th,
  td {
    padding: .72rem .66rem;
    font-size: .84rem;
  }

  .preview {
    max-width: 220px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .btn-detail {
    min-width: 64px;
    height: 28px;
    font-size: .75rem;
  }

  .modal-mask {
    align-items: flex-start;
    padding: .6rem;
  }

  .modal-panel {
    width: 100%;
    max-height: 92dvh;
  }

  .modal-header {
    align-items: flex-start;
    gap: .65rem;
  }

  .modal-header h3 {
    font-size: .95rem;
    line-height: 1.35;
  }

  .detail-table.wide {
    min-width: 640px;
  }
}

@media (max-width: 430px) {
  .site-header {
    padding: .78rem .78rem;
  }

  .site-header nav a {
    font-size: .8rem;
  }

  .container {
    padding: 0 .75rem;
  }

  table {
    min-width: 580px;
  }

  th,
  td {
    padding: .62rem .56rem;
    font-size: .8rem;
  }

  .title {
    font-size: .84rem;
  }

  .name {
    font-size: .72rem;
  }

  .preview {
    max-width: 180px;
  }

  .detail-meta {
    gap: .45rem;
    font-size: .78rem;
  }

  .detail-table.wide {
    min-width: 560px;
  }
}

@media (max-width: 390px) {
  table {
    min-width: 560px;
  }

  .preview {
    max-width: 155px;
  }

  .detail-table.wide {
    min-width: 540px;
  }
}

@media (max-width: 375px) {
  .site-header h1 {
    font-size: 1.02rem;
  }

  table {
    min-width: 540px;
  }

  .detail-table.wide {
    min-width: 520px;
  }
}

</style>
