<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

interface FormStat {
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
  IsExpired?: boolean
  FieldCount: number
  DataCount: number
}

interface AdminSummary {
  total: number
  visible: number
  pinnedCount: number
  expiredCount: number
  byStatus: Record<'published' | 'draft' | 'archived', number>
  byCategory: Record<string, number>
}

const forms = ref<FormStat[]>([])
const user = ref<{ Username: string } | null>(null)
const loading = ref(true)
const error = ref('')
const selectedStatus = ref<'all' | 'published' | 'draft' | 'archived'>('all')
const selectedCategory = ref('all')
const keyword = ref('')
const includeExpired = ref(true)
const categoryOptions = ref<string[]>([])
const summary = ref<AdminSummary>({
  total: 0,
  visible: 0,
  pinnedCount: 0,
  expiredCount: 0,
  byStatus: {
    published: 0,
    draft: 0,
    archived: 0,
  },
  byCategory: {},
})
const showDataModal = ref(false)
const dataLoading = ref(false)
const dataError = ref('')
const currentFormTitle = ref('')
const dataFields = ref<Array<{ Name: string; Label: string }>>([])
const dataRows = ref<Array<Record<string, any>>>([])
const showShareModal = ref(false)
const shareLoading = ref(false)
const shareError = ref('')
const shareFormTitle = ref('')
const generatedShareURL = ref('')
const generatedShareExpireAt = ref('')
const shareCopied = ref(false)
const showEditModal = ref(false)
const editLoading = ref(false)
const editSaving = ref(false)
const editError = ref('')
const editContent = ref('')
const editSourceFile = ref('')
const editFormName = ref('')
const editSaveResult = ref('')
const viewportWidth = ref(9999)
const router = useRouter()
const auth = useAuthStore()

const MOBILE_BREAKPOINT = 430
const COMPACT_BREAKPOINT = 520

const isMobile = computed(() => viewportWidth.value <= MOBILE_BREAKPOINT)
const isCompactPhone = computed(
  () => viewportWidth.value > MOBILE_BREAKPOINT && viewportWidth.value <= COMPACT_BREAKPOINT,
)
const hasActiveFilters = computed(
  () =>
    selectedStatus.value !== 'all' ||
    selectedCategory.value !== 'all' ||
    keyword.value.trim() !== '' ||
    !includeExpired.value,
)

function updateViewportMode() {
  viewportWidth.value = window.innerWidth
}

function buildAdminQuery() {
  const params = new URLSearchParams()
  if (selectedStatus.value !== 'all') {
    params.set('status', selectedStatus.value)
  }
  if (selectedCategory.value !== 'all') {
    params.set('category', selectedCategory.value)
  }
  const trimmedKeyword = keyword.value.trim()
  if (trimmedKeyword) {
    params.set('keyword', trimmedKeyword)
  }
  if (!includeExpired.value) {
    params.set('include_expired', 'false')
  }
  return params.toString()
}

async function fetchAdminData() {
  loading.value = true
  error.value = ''

  try {
    const query = buildAdminQuery()
    const res = await fetch(query ? `/api/admin?${query}` : '/api/admin')
    if (res.status === 401) {
      router.push('/login')
      return
    }
    if (!res.ok) throw new Error('加载失败')

    const data = await res.json()
    forms.value = data.forms ?? []
    user.value = data.user ?? null
    categoryOptions.value = data.availableCategories ?? []

    summary.value = {
      total: data.summary?.total ?? 0,
      visible: data.summary?.visible ?? 0,
      pinnedCount: data.summary?.pinnedCount ?? 0,
      expiredCount: data.summary?.expiredCount ?? 0,
      byStatus: {
        published: data.summary?.byStatus?.published ?? 0,
        draft: data.summary?.byStatus?.draft ?? 0,
        archived: data.summary?.byStatus?.archived ?? 0,
      },
      byCategory: data.summary?.byCategory ?? {},
    }
  } catch (e: any) {
    error.value = e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  fetchAdminData()
}

function resetFilters() {
  selectedStatus.value = 'all'
  selectedCategory.value = 'all'
  keyword.value = ''
  includeExpired.value = true
  fetchAdminData()
}

onMounted(async () => {
  updateViewportMode()
  window.addEventListener('resize', updateViewportMode)

  await fetchAdminData()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewportMode)
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

async function generateShareLink(form: FormStat) {
  showShareModal.value = true
  shareLoading.value = true
  shareError.value = ''
  shareCopied.value = false
  shareFormTitle.value = form.Title
  generatedShareURL.value = ''
  generatedShareExpireAt.value = form.ExpireAt ?? ''

  try {
    const res = await fetch('/api/admin/share-links', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ formName: form.Name }),
    })
    if (!res.ok) {
      const payload = await res.json().catch(() => ({}))
      throw new Error(payload.error || '生成链接失败')
    }
    const payload = await res.json()
    generatedShareURL.value = payload.url ?? ''
    generatedShareExpireAt.value = payload.expireAt ?? ''
  } catch (e: any) {
    shareError.value = e.message || '生成链接失败'
  } finally {
    shareLoading.value = false
  }
}

async function copyShareLink() {
  if (!generatedShareURL.value) return
  shareError.value = ''

  // Clipboard API requires secure contexts. Fallback keeps copy usable on intranet HTTP.
  const fallbackCopy = (text: string) => {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.setAttribute('readonly', 'readonly')
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    textarea.setSelectionRange(0, textarea.value.length)
    let copied = false
    try {
      copied = document.execCommand('copy')
    } finally {
      document.body.removeChild(textarea)
    }
    return copied
  }

  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(generatedShareURL.value)
      shareCopied.value = true
      return
    }

    shareCopied.value = fallbackCopy(generatedShareURL.value)
  } catch {
    shareCopied.value = fallbackCopy(generatedShareURL.value)
  }

  if (!shareCopied.value) {
    shareCopied.value = false
    shareError.value = '复制失败，请手动选择链接并复制。'
  }
}

function closeShareModal() {
  showShareModal.value = false
}

async function openEditModal(form: FormStat) {
  showEditModal.value = true
  editLoading.value = true
  editError.value = ''
  editSaveResult.value = ''
  editFormName.value = form.Name
  editContent.value = ''
  editSourceFile.value = ''

  try {
    const res = await fetch(`/api/admin/form-config/${form.Name}`)
    if (!res.ok) {
      const payload = await res.json().catch(() => ({}))
      throw new Error(payload.error || '加载配置失败')
    }
    const payload = await res.json()
    editContent.value = payload.content ?? ''
    editSourceFile.value = payload.source ?? ''
  } catch (e: any) {
    editError.value = e.message || '加载失败'
  } finally {
    editLoading.value = false
  }
}

async function saveFormConfig() {
  editSaving.value = true
  editError.value = ''
  editSaveResult.value = ''

  try {
    const res = await fetch(`/api/admin/form-config/${editFormName.value}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: editContent.value }),
    })
    const payload = await res.json().catch(() => ({}))
    if (!res.ok) {
      throw new Error(payload.error || '保存失败')
    }
    editSaveResult.value = payload.message || '配置已保存并重载'
    await fetchAdminData()
  } catch (e: any) {
    editError.value = e.message || '保存失败'
  } finally {
    editSaving.value = false
  }
}

function closeEditModal() {
  showEditModal.value = false
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
        <section class="filter-panel">
          <label class="filter-field">
            <span>状态</span>
            <select v-model="selectedStatus">
              <option value="all">全部</option>
              <option value="published">已发布</option>
              <option value="draft">草稿</option>
              <option value="archived">归档</option>
            </select>
          </label>

          <label class="filter-field">
            <span>分类</span>
            <select v-model="selectedCategory">
              <option value="all">全部</option>
              <option v-for="category in categoryOptions" :key="category" :value="category">
                {{ category }}
              </option>
            </select>
          </label>

          <label class="filter-field filter-keyword">
            <span>关键词</span>
            <input v-model="keyword" type="text" placeholder="名称 / 标题 / 描述" @keyup.enter="applyFilters" />
          </label>

          <label class="filter-check">
            <input v-model="includeExpired" type="checkbox" />
            <span>包含已过期表单</span>
          </label>

          <div class="filter-actions">
            <button class="btn-apply" @click="applyFilters">应用筛选</button>
            <button class="btn-reset" :disabled="!hasActiveFilters" @click="resetFilters">重置</button>
          </div>
        </section>

        <section class="summary-grid">
          <article class="summary-card">
            <p class="summary-label">可见 / 全部</p>
            <p class="summary-value">{{ summary.visible }} / {{ summary.total }}</p>
          </article>
          <article class="summary-card">
            <p class="summary-label">已发布</p>
            <p class="summary-value">{{ summary.byStatus.published }}</p>
          </article>
          <article class="summary-card">
            <p class="summary-label">草稿 / 归档</p>
            <p class="summary-value">{{ summary.byStatus.draft }} / {{ summary.byStatus.archived }}</p>
          </article>
          <article class="summary-card">
            <p class="summary-label">置顶 / 已过期</p>
            <p class="summary-value">{{ summary.pinnedCount }} / {{ summary.expiredCount }}</p>
          </article>
        </section>

        <div v-if="!isMobile && !isCompactPhone" class="table-wrap">
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
                  <div class="form-meta">
                    <span>{{ form.Category || 'general' }}</span>
                    <span>{{ form.Status || 'published' }}</span>
                    <span>序 {{ form.SortOrder ?? 0 }}</span>
                    <span v-if="form.Pinned">置顶</span>
                    <span v-if="form.IsExpired">已过期</span>
                  </div>
                </td>
                <td class="num-cell">{{ form.FieldCount }}</td>
                <td class="num-cell">
                  <span class="badge">{{ form.DataCount }}</span>
                </td>
                <td class="actions-cell">
                  <div class="actions-group">
                    <button class="btn-view-data" @click="viewData(form)">查看</button>
                    <button class="btn-view" @click="router.push(`/forms/${form.Name}`)">填写</button>
                    <button class="btn-share" @click="generateShareLink(form)">专用链接</button>
                    <button class="btn-edit" @click="openEditModal(form)">编辑</button>
                    <button class="btn-export" @click="exportCSV(form.Name)">导出 CSV</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="mobile-list" :class="{ 'mobile-list-compact': isCompactPhone }">
          <article v-for="form in forms" :key="`mobile-${form.Name}`" class="mobile-card">
            <div class="mobile-title">{{ form.Title }}</div>
            <div class="mobile-slug">{{ form.Name }}</div>
            <div class="mobile-meta">
              <span>{{ form.Category || 'general' }}</span>
              <span>{{ form.Status || 'published' }}</span>
              <span>序 {{ form.SortOrder ?? 0 }}</span>
              <span v-if="form.Pinned">置顶</span>
              <span v-if="form.IsExpired">已过期</span>
            </div>
            <div class="mobile-stats">
              <span>字段数：{{ form.FieldCount }}</span>
              <span>提交数：{{ form.DataCount }}</span>
            </div>
            <div class="mobile-actions">
              <button class="btn-view-data" @click="viewData(form)">查看</button>
              <button class="btn-view" @click="router.push(`/forms/${form.Name}`)">填写</button>
              <button class="btn-share" @click="generateShareLink(form)">专用链接</button>
              <button class="btn-edit" @click="openEditModal(form)">编辑</button>
              <button class="btn-export" @click="exportCSV(form.Name)">导出</button>
            </div>
          </article>
        </div>
      </div>

      <div v-if="showShareModal" class="modal-mask" @click.self="closeShareModal">
        <div class="modal-panel share-panel">
          <div class="modal-header">
            <h3>{{ shareFormTitle }} - 专用填写链接</h3>
            <button class="btn-close" @click="closeShareModal">关闭</button>
          </div>

          <div class="modal-body">
            <div v-if="shareLoading" class="state-msg">生成中…</div>
            <div v-else-if="shareError" class="state-msg error">{{ shareError }}</div>
            <div v-else class="share-body">
              <p class="share-tip">该链接可让用户直接填写当前表单，无需进入表单列表。</p>
              <div class="share-link-box">{{ generatedShareURL }}</div>
              <p class="share-expire">表单截止：{{ generatedShareExpireAt || '长期有效' }}</p>
              <div class="share-actions">
                <button class="btn-share-copy" @click="copyShareLink">复制链接</button>
                <span v-if="shareCopied" class="share-copied">已复制</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="showEditModal" class="modal-mask" @click.self="closeEditModal">
        <div class="modal-panel edit-panel">
          <div class="modal-header">
            <div class="edit-header-info">
              <h3>编辑配置 — {{ editFormName }}</h3>
              <span v-if="editSourceFile" class="edit-source-file">{{ editSourceFile }}</span>
            </div>
            <button class="btn-close" @click="closeEditModal">关闭</button>
          </div>

          <div class="modal-body edit-modal-body">
            <div v-if="editLoading" class="state-msg">加载配置中…</div>
            <div v-else-if="editError && !editContent" class="state-msg error">{{ editError }}</div>
            <template v-else>
              <textarea
                v-model="editContent"
                class="yaml-editor"
                spellcheck="false"
                autocomplete="off"
                autocorrect="off"
                autocapitalize="off"
              />
              <div class="edit-footer">
                <div class="edit-messages">
                  <span v-if="editError" class="edit-error">{{ editError }}</span>
                  <span v-else-if="editSaveResult" class="edit-success">{{ editSaveResult }}</span>
                </div>
                <div class="edit-footer-actions">
                  <button class="btn-close" @click="closeEditModal">取消</button>
                  <button class="btn-save-config" :disabled="editSaving" @click="saveFormConfig">
                    {{ editSaving ? '保存中…' : '保存并重载' }}
                  </button>
                </div>
              </div>
            </template>
          </div>
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
.page {
  min-height: 100vh;
  min-height: 100dvh;
  background: transparent;
}

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

.filter-panel {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: .65rem;
  padding: .72rem;
  margin-bottom: .82rem;
  border: 1px solid var(--surface-card-border);
  border-radius: 12px;
  background: linear-gradient(180deg, #ffffff 0%, #f8faff 100%);
}

.filter-field {
  grid-column: span 2;
  display: grid;
  gap: .3rem;
}

.filter-field span,
.filter-check span {
  font-size: .74rem;
  color: #68758f;
}

.filter-field select,
.filter-field input {
  height: 34px;
  border-radius: 8px;
  border: 1px solid #d9e1f1;
  padding: 0 .62rem;
  font-size: .83rem;
  color: #1f2937;
  background: #fff;
}

.filter-keyword {
  grid-column: span 4;
}

.filter-check {
  grid-column: span 2;
  display: flex;
  align-items: center;
  align-self: end;
  height: 34px;
  gap: .45rem;
  white-space: nowrap;
}

.filter-check input {
  margin: 0;
  flex-shrink: 0;
}

.filter-check span {
  line-height: 1;
  white-space: nowrap;
}

.filter-actions {
  grid-column: span 2;
  display: flex;
  gap: .46rem;
  justify-content: flex-end;
  align-items: end;
}

.btn-apply,
.btn-reset {
  height: 34px;
  padding: 0 .72rem;
  border-radius: 8px;
  border: none;
  font-size: .8rem;
  font-weight: 600;
  cursor: pointer;
}

.btn-apply {
  background: #e8efff;
  color: #2250bb;
}

.btn-reset {
  background: #f3f5fa;
  color: #4b5568;
}

.btn-reset:disabled {
  opacity: .6;
  cursor: not-allowed;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: .6rem;
  margin-bottom: .88rem;
}

.summary-card {
  border: 1px solid var(--surface-card-border);
  border-radius: 10px;
  background: linear-gradient(180deg, #ffffff 0%, #f7fafe 100%);
  padding: .62rem .72rem;
}

.summary-label {
  margin: 0;
  font-size: .74rem;
  color: #6a7389;
}

.summary-value {
  margin: .28rem 0 0;
  font-size: 1.08rem;
  font-weight: 700;
  color: #1f2937;
}

.table-wrap {
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  border-radius: 12px;
  border: 1px solid var(--surface-card-border);
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  box-shadow: var(--shadow-soft);
}
table {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
}

.col-name { width: auto; }
.col-num { width: 120px; }
.col-action { width: 460px; }

th {
  background: #f6f8fd;
  padding: .9rem 1rem;
  text-align: left;
  font-size: .85rem;
  font-weight: 600;
  letter-spacing: .01em;
  white-space: nowrap;
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

.form-meta {
  margin-top: .3rem;
  display: flex;
  flex-wrap: wrap;
  gap: .28rem;
}

.form-meta span {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 .42rem;
  border-radius: 999px;
  border: 1px solid #dfe6f4;
  background: #f8faff;
  color: #596783;
  font-size: .68rem;
}

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

.btn-view-data, .btn-view, .btn-share, .btn-edit, .btn-export {
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
.btn-share { background: #f0ecff; color: #5b43b8; }
.btn-edit { background: #e8f8ef; color: #1d7a47; }
.btn-export { background: var(--bg-soft-green); color: var(--status-success); }
.btn-view-data:hover, .btn-view:hover, .btn-share:hover, .btn-edit:hover, .btn-export:hover {
  opacity: .95;
  transform: translateY(-1px);
  box-shadow: 0 3px 10px rgba(17, 24, 39, .08);
}

.btn-view-data:focus-visible,
.btn-view:focus-visible,
.btn-share:focus-visible,
.btn-edit:focus-visible,
.btn-export:focus-visible {
  box-shadow: 0 0 0 3px rgba(37, 99, 235, .2);
}

.share-panel {
  width: min(760px, 96vw);
}

.share-body {
  display: grid;
  gap: .58rem;
}

.share-tip {
  margin: 0;
  font-size: .84rem;
  color: #5f6f8b;
}

.share-link-box {
  border: 1px solid #d8e2f5;
  background: #f8fbff;
  border-radius: 10px;
  padding: .62rem .72rem;
  font-size: .84rem;
  color: #2b3c5e;
  word-break: break-all;
}

.share-expire {
  margin: 0;
  font-size: .8rem;
  color: #65758f;
}

.share-actions {
  display: flex;
  align-items: center;
  gap: .5rem;
}

.btn-share-copy {
  border: none;
  background: #e8efff;
  color: #2250bb;
  height: 34px;
  padding: 0 .78rem;
  border-radius: 8px;
  font-size: .8rem;
  font-weight: 600;
  cursor: pointer;
}

.share-copied {
  font-size: .78rem;
  color: #2f7d4f;
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

.mobile-list {
  display: grid;
  gap: .7rem;
}

.mobile-card {
  border: 1px solid var(--surface-card-border);
  border-radius: 12px;
  background: linear-gradient(180deg, var(--surface-card-start) 0%, var(--surface-card-end) 100%);
  box-shadow: var(--shadow-soft);
  padding: .72rem;
}

.mobile-title {
  font-size: .96rem;
  font-weight: 600;
  color: #1a1a2e;
}

.mobile-slug {
  margin-top: .16rem;
  font-size: .75rem;
  color: #8a93a8;
}

.mobile-stats {
  margin-top: .52rem;
  display: flex;
  flex-wrap: wrap;
  gap: .36rem .9rem;
  font-size: .8rem;
  color: #4b5568;
}

.mobile-meta {
  margin-top: .32rem;
  display: flex;
  flex-wrap: wrap;
  gap: .28rem;
}

.mobile-meta span {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 .42rem;
  border-radius: 999px;
  border: 1px solid #dfe6f4;
  background: #f8faff;
  color: #596783;
  font-size: .68rem;
}

.mobile-actions {
  margin-top: .58rem;
  display: flex;
  flex-wrap: wrap;
  gap: .36rem;
}

.mobile-list-compact {
  gap: .56rem;
}

.mobile-list-compact .mobile-card {
  padding: .62rem;
}

.mobile-list-compact .mobile-title {
  font-size: .9rem;
}

.mobile-list-compact .mobile-slug {
  font-size: .72rem;
}

.mobile-list-compact .mobile-stats {
  margin-top: .42rem;
  font-size: .76rem;
}

.mobile-list-compact .mobile-actions {
  margin-top: .48rem;
  gap: .3rem;
}

.mobile-list-compact .btn-view-data,
.mobile-list-compact .btn-view,
.mobile-list-compact .btn-share,
.mobile-list-compact .btn-edit,
.mobile-list-compact .btn-export {
  min-width: 64px;
  height: 28px;
  padding: 0 .46rem;
  font-size: .72rem;
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

  .header-right {
    width: 100%;
    flex-wrap: wrap;
    gap: .5rem .7rem;
  }

  .container {
    margin: 1rem auto;
  }

  .filter-panel {
    grid-template-columns: repeat(6, minmax(0, 1fr));
    padding: .62rem;
  }

  .filter-field {
    grid-column: span 3;
  }

  .filter-keyword {
    grid-column: span 6;
  }

  .filter-check {
    grid-column: span 3;
  }

  .filter-actions {
    grid-column: span 3;
  }

  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  table {
    min-width: 680px;
  }

  .col-num {
    width: 96px;
  }

  .col-action {
    width: 380px;
  }

  th,
  td {
    padding: .72rem .66rem;
  }

  .actions-group {
    gap: .38rem;
  }

  .btn-view-data,
  .btn-view,
  .btn-share,
  .btn-edit,
  .btn-export {
    min-width: 66px;
    height: 30px;
    padding: 0 .52rem;
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

  .data-table {
    min-width: 640px;
  }
}

@media (max-width: 430px) {
  .site-header {
    padding: .78rem .78rem;
  }

  .header-right {
    gap: .45rem .62rem;
  }

  .user-badge,
  .link,
  .btn-logout {
    font-size: .76rem;
  }

  .btn-logout {
    padding: .28rem .62rem;
  }

  .btn-view-data,
  .btn-view,
  .btn-share,
  .btn-edit,
  .btn-export {
    min-width: 68px;
    height: 28px;
    font-size: .72rem;
    padding: 0 .45rem;
  }

  .data-table {
    min-width: 560px;
  }

  .filter-panel {
    grid-template-columns: 1fr;
    gap: .52rem;
  }

  .filter-field,
  .filter-keyword,
  .filter-check,
  .filter-actions {
    grid-column: auto;
  }

  .filter-actions {
    justify-content: flex-start;
  }

  .summary-grid {
    grid-template-columns: 1fr;
  }

  .summary-value {
    font-size: 1rem;
  }

  .data-table th,
  .data-table td {
    font-size: .78rem;
    padding: .56rem .5rem;
  }
}

@media (max-width: 390px) {
  .actions-group {
    gap: .3rem;
  }

  .btn-view-data,
  .btn-view,
  .btn-share,
  .btn-edit,
  .btn-export {
    min-width: 60px;
    font-size: .7rem;
    padding: 0 .38rem;
  }

  .data-table {
    min-width: 540px;
  }
}

@media (max-width: 375px) {
  .site-header h1 {
    font-size: 1.02rem;
  }

  .header-right {
    gap: .35rem .52rem;
  }

  .data-table {
    min-width: 520px;
  }
}

/* ---- YAML editor modal ---- */
.edit-panel {
  width: min(860px, 96vw);
  max-height: 90dvh;
  display: flex;
  flex-direction: column;
}

.edit-header-info {
  display: flex;
  flex-direction: column;
  gap: .18rem;
}

.edit-source-file {
  font-size: .74rem;
  color: #64748b;
  font-family: ui-monospace, 'Cascadia Code', monospace;
}

.edit-modal-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
  padding-bottom: 0;
}

.yaml-editor {
  flex: 1;
  width: 100%;
  min-height: 340px;
  max-height: calc(90dvh - 180px);
  resize: vertical;
  font-family: ui-monospace, 'Cascadia Code', 'Courier New', monospace;
  font-size: .84rem;
  line-height: 1.6;
  padding: .72rem .78rem;
  border: 1px solid #d1d9ed;
  border-radius: 8px;
  background: #f8fafc;
  color: #1e293b;
  tab-size: 2;
  white-space: pre;
  overflow-wrap: normal;
  overflow-x: auto;
  box-sizing: border-box;
}

.yaml-editor:focus {
  outline: none;
  border-color: #93aee8;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, .12);
  background: #fff;
}

.edit-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: .5rem;
  padding: .62rem 0 .3rem;
  flex-shrink: 0;
}

.edit-messages {
  flex: 1;
  min-width: 0;
}

.edit-error {
  font-size: .81rem;
  color: #e53e3e;
  word-break: break-word;
}

.edit-success {
  font-size: .81rem;
  color: #2d7a4f;
}

.edit-footer-actions {
  display: flex;
  gap: .46rem;
  flex-shrink: 0;
}

.btn-save-config {
  height: 34px;
  padding: 0 1rem;
  border-radius: 8px;
  border: none;
  background: #2250bb;
  color: #fff;
  font-size: .84rem;
  font-weight: 600;
  cursor: pointer;
  transition: opacity .18s;
}

.btn-save-config:disabled {
  opacity: .6;
  cursor: not-allowed;
}
</style>
