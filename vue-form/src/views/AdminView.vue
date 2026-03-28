<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

interface FormStat {
  Name: string
  Title: string
  Description: string
  FieldCount: number
  DataCount: number
}

const forms = ref<FormStat[]>([])
const user = ref<{ Username: string } | null>(null)
const loading = ref(true)
const error = ref('')
const showDataModal = ref(false)
const dataLoading = ref(false)
const dataError = ref('')
const currentFormTitle = ref('')
const dataFields = ref<Array<{ Name: string; Label: string }>>([])
const dataRows = ref<Array<Record<string, any>>>([])
const router = useRouter()
const auth = useAuthStore()

onMounted(async () => {
  try {
    const res = await fetch('/api/admin')
    if (res.status === 401) {
      router.push('/login')
      return
    }
    if (!res.ok) throw new Error('加载失败')
    const data = await res.json()
    forms.value = data.forms ?? []
    user.value = data.user
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

function exportCSV(formName: string) {
  window.location.href = `/api/export/${formName}`
}

function normalizeCellValue(value: unknown): string {
  if (Array.isArray(value)) return value.join(', ')
  if (value === null || value === undefined || value === '') return '-'
  return String(value)
}

async function viewData(form: FormStat) {
  showDataModal.value = true
  dataLoading.value = true
  dataError.value = ''
  currentFormTitle.value = form.Title
  dataFields.value = []
  dataRows.value = []

  try {
    const res = await fetch(`/api/data/${form.Name}`)
    if (!res.ok) throw new Error('加载数据失败')
    const payload = await res.json()
    dataFields.value = payload.fields ?? []
    dataRows.value = payload.data ?? []
  } catch (e: any) {
    dataError.value = e.message || '加载失败'
  } finally {
    dataLoading.value = false
  }
}

function closeDataModal() {
  showDataModal.value = false
}
</script>

<template>
  <div class="page">
    <header class="site-header">
      <h1>管理后台</h1>
      <div class="header-right">
        <span v-if="user" class="user-badge">{{ user.Username }}</span>
        <a href="/admin/users" @click.prevent="router.push('/admin/users')" class="link">用户管理</a>
        <button class="btn-logout" @click="logout">退出登录</button>
        <a href="/" @click.prevent="router.push('/')" class="link">← 前台首页</a>
      </div>
    </header>

    <main class="container">
      <div v-if="loading" class="state-msg">加载中…</div>
      <div v-else-if="error" class="state-msg error">{{ error }}</div>

      <div v-else>
        <h2 class="section-title">表单数据统计</h2>
        <div class="table-wrap">
          <table>
            <colgroup>
              <col class="col-name" />
              <col class="col-num" />
              <col class="col-num" />
              <col class="col-action" />
            </colgroup>
            <thead>
              <tr>
                <th class="th-name">表单名称</th>
                <th class="th-num">字段数</th>
                <th class="th-num">提交数</th>
                <th class="th-action">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="form in forms" :key="form.Name">
                <td>
                  <div class="form-name">{{ form.Title }}</div>
                  <div class="form-slug">{{ form.Name }}</div>
                </td>
                <td class="num-cell">{{ form.FieldCount }}</td>
                <td class="num-cell">
                  <span class="badge">{{ form.DataCount }}</span>
                </td>
                <td class="actions-cell">
                  <div class="actions-group">
                    <button class="btn-view-data" @click="viewData(form)">查看</button>
                    <button class="btn-view" @click="router.push(`/forms/${form.Name}`)">填写</button>
                    <button class="btn-export" @click="exportCSV(form.Name)">导出 CSV</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="showDataModal" class="modal-mask" @click.self="closeDataModal">
        <div class="modal-panel">
          <div class="modal-header">
            <h3>{{ currentFormTitle }} - 收集数据</h3>
            <button class="btn-close" @click="closeDataModal">关闭</button>
          </div>

          <div class="modal-body">
            <div v-if="dataLoading" class="state-msg">加载中…</div>
            <div v-else-if="dataError" class="state-msg error">{{ dataError }}</div>
            <div v-else-if="dataRows.length === 0" class="state-msg">暂无数据</div>

            <div v-else class="data-table-wrap">
              <table class="data-table">
                <thead>
                  <tr>
                    <th v-for="field in dataFields" :key="field.Name">{{ field.Label }}</th>
                    <th>提交时间</th>
                    <th>IP</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, idx) in dataRows" :key="idx">
                    <td v-for="field in dataFields" :key="field.Name">
                      {{ normalizeCellValue(row[field.Name]) }}
                    </td>
                    <td>{{ normalizeCellValue(row['_submitted_at']) }}</td>
                    <td>{{ normalizeCellValue(row['_ip']) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
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
.header-right { display: flex; align-items: center; gap: 1rem; }
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
  transition: background .2s;
}
.btn-logout:hover { background: rgba(255,255,255,.1); }
.link { color: rgba(255,255,255,.7); text-decoration: none; font-size: .85rem; }

.container { max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
.state-msg { text-align: center; color: #888; padding: 3rem 0; }
.state-msg.error { color: #e53e3e; }

.section-title { font-size: 1rem; color: #444; margin-bottom: 1rem; }

.table-wrap {
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  border-radius: 12px;
  border: 1px solid var(--surface-card-border);
  overflow: hidden;
  box-shadow: var(--shadow-soft);
}
table { width: 100%; border-collapse: collapse; }

.col-name { width: auto; }
.col-num { width: 120px; }
.col-action { width: 390px; }

th {
  background: #f6f8fd;
  padding: .9rem 1rem;
  text-align: left;
  font-size: .85rem;
  font-weight: 600;
  letter-spacing: .01em;
  color: #5f6880;
  border-bottom: 1px solid #e6ebf3;
}

.th-num {
  text-align: center;
}

.th-action {
  text-align: right;
}

td {
  padding: .9rem 1rem;
  border-bottom: 1px solid #edf1f7;
  font-size: .9rem;
  vertical-align: middle;
}

.num-cell {
  text-align: center;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

tr:last-child td { border-bottom: none; }
tr:hover td { background: #f9fbff; }

.form-name { font-weight: 500; color: #1a1a2e; }
.form-slug { font-size: .78rem; color: #999; margin-top: .15rem; }

.badge {
  display: inline-block;
  background: var(--bg-soft-blue);
  color: var(--brand-600);
  padding: .2rem .65rem;
  border-radius: 20px;
  font-size: .85rem;
  font-weight: 500;
}

.actions-cell {
  white-space: nowrap;
  text-align: right;
}

.actions-group {
  display: inline-grid;
  grid-auto-flow: column;
  justify-content: end;
  align-items: center;
  gap: .5rem;
}

.btn-view-data, .btn-view, .btn-export {
  min-width: 86px;
  height: 34px;
  padding: 0 .75rem;
  border-radius: 6px;
  font-size: .82rem;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  border: none;
  outline: none;
  transition: transform .15s ease, box-shadow .15s ease, opacity .2s;
}
.btn-view-data { background: #fff4e6; color: #c76b00; }
.btn-view { background: var(--bg-soft-blue); color: var(--brand-600); }
.btn-export { background: var(--bg-soft-green); color: var(--status-success); }
.btn-view-data:hover, .btn-view:hover, .btn-export:hover {
  opacity: .95;
  transform: translateY(-1px);
  box-shadow: 0 3px 10px rgba(17, 24, 39, .08);
}

.btn-view-data:focus-visible,
.btn-view:focus-visible,
.btn-export:focus-visible {
  box-shadow: 0 0 0 3px rgba(37, 99, 235, .2);
}

.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(21, 30, 53, .38);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  z-index: 40;
}

.modal-panel {
  width: min(1180px, 96vw);
  max-height: 86vh;
  background: #ffffff;
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
  font-weight: 600;
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

.modal-body {
  padding: .8rem 1rem 1rem;
  overflow: auto;
}

.data-table-wrap {
  overflow: auto;
  border: 1px solid #e9eef6;
  border-radius: 8px;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  min-width: 760px;
}

.data-table th,
.data-table td {
  padding: .62rem .7rem;
  border-bottom: 1px solid #eef2f7;
  text-align: left;
  font-size: .83rem;
  vertical-align: top;
}

.data-table th {
  position: sticky;
  top: 0;
  background: #f6f8fd;
  font-weight: 600;
  color: #475569;
  z-index: 1;
}

.data-table tbody tr:hover td {
  background: #f8fbff;
}
</style>
