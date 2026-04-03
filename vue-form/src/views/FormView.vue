<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

interface Field {
  Name: string
  Label: string
  Type: string
  Placeholder: string
  Required: boolean
  Options: string[] | null
  Min: number | null
  Max: number | null
  Step: number | null
}

interface FormDef {
  Name: string
  Title: string
  Description: string
  Fields: Field[]
}

const route = useRoute()
const router = useRouter()
const formDef = ref<FormDef | null>(null)
const formData = ref<Record<string, any>>({})
const loading = ref(true)
const submitting = ref(false)
const submitted = ref(false)
const error = ref('')
const submitError = ref('')
const runningDistanceHint = ref('支持输入小数，最多 3 位，例如 21.097')
const runningDistanceError = ref('')

function isShareMode(): boolean {
  return typeof route.params.token === 'string' && route.params.token.trim() !== ''
}

function getFormFetchPath(): string {
  if (isShareMode()) {
    return `/api/public/forms/${route.params.token}`
  }
  return `/api/forms/${route.params.formName}`
}

function getSubmitPath(): string {
  if (isShareMode()) {
    return `/api/public/submit/${route.params.token}`
  }
  return `/api/submit/${route.params.formName}`
}

function parseDurationToSeconds(raw: unknown): number | null {
  if (typeof raw !== 'string') return null
  const value = raw.trim()
  if (!value) return null

  const parts = value.split(':').map((p) => p.trim())
  if (parts.length !== 2 && parts.length !== 3) return null
  if (parts.some((p) => p === '' || !/^\d+$/.test(p))) return null

  if (parts.length === 2) {
    const minutes = Number(parts[0])
    const seconds = Number(parts[1])
    if (seconds >= 60) return null
    return minutes * 60 + seconds
  }

  const hours = Number(parts[0])
  const minutes = Number(parts[1])
  const seconds = Number(parts[2])
  if (minutes >= 60 || seconds >= 60) return null
  return hours * 3600 + minutes * 60 + seconds
}

function updateAveragePace() {
  if (!formDef.value) return

  const hasPaceFields = formDef.value.Fields.some((f) => f.Name === 'running_distance')
    && formDef.value.Fields.some((f) => f.Name === 'total_time')
    && formDef.value.Fields.some((f) => f.Name === 'average_pace')
  if (!hasPaceFields) return

  const distanceRaw = formData.value.running_distance
  const distance = typeof distanceRaw === 'number' ? distanceRaw : Number(distanceRaw)
  const totalSeconds = parseDurationToSeconds(formData.value.total_time)

  if (!Number.isFinite(distance) || distance <= 0 || totalSeconds === null || totalSeconds <= 0) {
    formData.value.average_pace = ''
    return
  }

  const paceSeconds = Math.round(totalSeconds / distance)
  const paceMinutes = Math.floor(paceSeconds / 60)
  const remainSeconds = paceSeconds % 60
  formData.value.average_pace = `${paceMinutes}:${String(remainSeconds).padStart(2, '0')}`
}

function isReadonlyField(field: Field): boolean {
  return field.Name === 'average_pace'
}

function isRunningDistanceField(field: Field): boolean {
  return field.Name === 'running_distance'
}

function getNumberStep(field: Field): string | number {
  if (isRunningDistanceField(field)) return '0.001'
  return field.Step ?? 'any'
}

function validateRunningDistance(raw: unknown): string {
  if (raw === '' || raw === null || raw === undefined) return ''
  const value = typeof raw === 'number' ? raw : Number(raw)
  if (!Number.isFinite(value)) return '跑步距离必须是有效数字'
  if (value <= 0) return '跑步距离必须大于 0'

  const scaled = value * 1000
  if (Math.abs(scaled-Math.round(scaled)) > 1e-9) {
    return '跑步距离最多保留 3 位小数'
  }
  return ''
}

function updateRunningDistanceFeedback() {
  const validationError = validateRunningDistance(formData.value.running_distance)
  runningDistanceError.value = validationError

  if (validationError) {
    runningDistanceHint.value = '示例：21.097（公里）'
    return
  }

  const raw = formData.value.running_distance
  if (raw === '' || raw === null || raw === undefined) {
    runningDistanceHint.value = '支持输入小数，最多 3 位，例如 21.097'
    return
  }

  const value = typeof raw === 'number' ? raw : Number(raw)
  if (!Number.isFinite(value) || value <= 0) {
    runningDistanceHint.value = '示例：21.097（公里）'
    return
  }
  runningDistanceHint.value = `当前距离：${value.toFixed(3)} 公里`
}

onMounted(async () => {
  try {
    const res = await fetch(getFormFetchPath())
    if (!res.ok) {
      if (res.status === 410) {
        const data = await res.json()
        throw new Error(data.error || '该表单已到期，停止收集')
      }
      if (res.status === 404) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || '分享链接无效或表单不存在')
      }
      throw new Error('表单不存在')
    }
    formDef.value = await res.json()
    // 初始化表单数据
    for (const f of formDef.value!.Fields) {
      if (f.Type === 'checkbox') {
        formData.value[f.Name] = []
      } else if (f.Type === 'range') {
        // slider 默认取最小值，确保初始就显示动态分值
        formData.value[f.Name] = f.Min ?? 0
      } else {
        formData.value[f.Name] = ''
      }
    }
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
})

watch(
  () => [formData.value.running_distance, formData.value.total_time, formDef.value?.Name],
  () => {
    updateRunningDistanceFeedback()
    updateAveragePace()
  },
)

async function submit() {
  submitError.value = ''
  updateRunningDistanceFeedback()
  if (runningDistanceError.value) {
    submitError.value = runningDistanceError.value
    return
  }
  submitting.value = true
  try {
    const res = await fetch(getSubmitPath(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(formData.value),
    })
    if (res.ok) {
      submitted.value = true
    } else {
      const data = await res.json()
      if (res.status === 410) {
        submitError.value = data.error || '该表单已到期，停止收集'
        return
      }
      if (res.status === 401) {
        submitError.value = data.error || '请先登录'
        if (!isShareMode()) {
          setTimeout(() => router.push('/login'), 700)
        }
        return
      }
      submitError.value = data.error || '提交失败，请重试'
    }
  } catch {
    submitError.value = '网络错误，请稍后重试'
  } finally {
    submitting.value = false
  }
}

function getRangeValue(field: Field): number {
  const raw = formData.value[field.Name]
  if (typeof raw === 'number' && !Number.isNaN(raw)) return raw
  if (typeof raw === 'string' && raw !== '') {
    const parsed = Number(raw)
    if (!Number.isNaN(parsed)) return parsed
  }
  return field.Min ?? 0
}

function getRangeBounds(field: Field): { min: number; max: number } {
  const rawMin = Number(field.Min ?? 0)
  const rawMax = Number(field.Max ?? 100)
  const min = Number.isFinite(rawMin) ? rawMin : 0
  const max = Number.isFinite(rawMax) ? rawMax : 100

  if (max <= min) {
    return { min, max: min + 1 }
  }
  return { min, max }
}

function getRangeTicks(field: Field): number[] {
  const { min, max } = getRangeBounds(field)
  const span = max - min
  const segments = span <= 10 && Number.isInteger(span) ? Math.max(1, Math.min(10, span)) : 5
  const step = span / segments

  return Array.from({ length: segments + 1 }, (_, i) => {
    const value = min + step * i
    return Number.isInteger(value) ? value : Number(value.toFixed(1))
  })
}
</script>

<template>
  <div class="page">
    <header class="site-header">
      <a href="/" @click.prevent="router.push('/')">← 返回首页</a>
    </header>

    <main class="container">
      <div v-if="loading" class="state-msg">加载中…</div>
      <div v-else-if="error" class="state-msg error">{{ error }}</div>

      <!-- 提交成功 -->
      <div v-else-if="submitted" class="success-card">
        <div class="success-icon">✓</div>
        <h2>提交成功！</h2>
        <p>感谢您的填写，数据已保存。</p>
        <button @click="router.push(isShareMode() ? '/login' : '/')">{{ isShareMode() ? '返回登录页' : '返回首页' }}</button>
      </div>

      <!-- 表单 -->
      <div v-else-if="formDef" class="form-card">
        <h1>{{ formDef.Title }}</h1>
        <p v-if="formDef.Description" class="desc">{{ formDef.Description }}</p>

        <form @submit.prevent="submit">
          <div
            v-for="field in formDef.Fields"
            :key="field.Name"
            class="field"
          >
            <label :for="field.Name">
              {{ field.Label }}
              <span v-if="field.Required" class="required">*</span>
              <span v-if="field.Type === 'range'" class="range-inline-value">
                {{ getRangeValue(field) }} / {{ getRangeBounds(field).max }}
              </span>
            </label>

            <!-- textarea -->
            <textarea
              v-if="field.Type === 'textarea'"
              :id="field.Name"
              v-model="formData[field.Name]"
              :placeholder="field.Placeholder"
              :required="field.Required"
              rows="4"
            />

            <!-- select -->
            <select
              v-else-if="field.Type === 'select'"
              :id="field.Name"
              v-model="formData[field.Name]"
              :required="field.Required"
            >
              <option value="">请选择…</option>
              <option v-for="opt in field.Options" :key="opt" :value="opt">{{ opt }}</option>
            </select>

            <!-- radio -->
            <div v-else-if="field.Type === 'radio'" class="option-group">
              <label v-for="opt in field.Options" :key="opt" class="option-label">
                <input type="radio" :name="field.Name" :value="opt" v-model="formData[field.Name]" :required="field.Required" />
                {{ opt }}
              </label>
            </div>

            <!-- checkbox -->
            <div v-else-if="field.Type === 'checkbox'" class="option-group">
              <label v-for="opt in field.Options" :key="opt" class="option-label">
                <input type="checkbox" :value="opt" v-model="formData[field.Name]" />
                {{ opt }}
              </label>
            </div>

            <!-- number -->
            <template v-else-if="field.Type === 'number'">
              <input
                :id="field.Name"
                type="number"
                v-model.number="formData[field.Name]"
                :placeholder="field.Placeholder"
                :required="field.Required"
                :min="field.Min ?? undefined"
                :max="field.Max ?? undefined"
                :step="getNumberStep(field)"
                :readonly="isReadonlyField(field)"
                :class="{ 'input-invalid': isRunningDistanceField(field) && !!runningDistanceError }"
              />
              <p v-if="isRunningDistanceField(field) && runningDistanceError" class="field-inline-error">
                {{ runningDistanceError }}
              </p>
              <p v-else-if="isRunningDistanceField(field)" class="field-inline-hint">
                {{ runningDistanceHint }}
              </p>
            </template>

            <!-- range slider -->
            <div v-else-if="field.Type === 'range'" class="range-wrap">
              <input
                :id="field.Name"
                type="range"
                v-model.number="formData[field.Name]"
                :required="field.Required"
                :min="getRangeBounds(field).min"
                :max="getRangeBounds(field).max"
              />

              <div class="range-ticks" aria-hidden="true">
                <span v-for="tick in getRangeTicks(field)" :key="`${field.Name}-${tick}`">{{ tick }}</span>
              </div>
            </div>

            <!-- email / tel / date / default text -->
            <input
              v-else
              :id="field.Name"
              :type="field.Type || 'text'"
              v-model="formData[field.Name]"
              :placeholder="field.Placeholder"
              :required="field.Required"
              :readonly="isReadonlyField(field)"
            />
          </div>

          <p v-if="submitError" class="error-msg">{{ submitError }}</p>

          <div class="actions">
            <button type="submit" :disabled="submitting" class="btn-submit">
              {{ submitting ? '提交中…' : '提交' }}
            </button>
          </div>
        </form>
      </div>
    </main>
  </div>
</template>

<style scoped>
.page { min-height: 100vh; background: transparent; }

.site-header {
  background: rgba(255, 255, 255, .74);
  border-bottom: 1px solid rgba(209, 213, 219, .7);
  backdrop-filter: blur(8px);
  padding: .9rem 2rem;
}
.site-header a { color: var(--brand-600); text-decoration: none; font-size: .9rem; }

.container { max-width: 640px; margin: 2rem auto; padding: 0 1rem; }

.state-msg { text-align: center; color: #888; padding: 4rem 0; }
.state-msg.error { color: #e53e3e; }

.success-card {
  text-align: center;
  background: linear-gradient(180deg, #ffffff 0%, #fcfefe 100%);
  border-radius: 16px;
  padding: 3rem 2rem;
  border: 1px solid #e6edf0;
}
.success-icon {
  width: 64px; height: 64px; border-radius: 50%;
  background: #48bb78; color: #fff; font-size: 2rem;
  display: flex; align-items: center; justify-content: center;
  margin: 0 auto 1rem;
}
.success-card h2 { margin: 0 0 .5rem; color: #1a1a2e; }
.success-card p { color: #666; margin-bottom: 1.5rem; }
.success-card button {
  background: var(--brand-600); color: #fff; border: none;
  padding: .65rem 1.5rem; border-radius: 8px; cursor: pointer; font-size: .95rem;
}

.form-card {
  background: linear-gradient(180deg, #ffffff 0%, #fcfdff 100%);
  border-radius: 16px;
  padding: 2rem;
  border: 1px solid #e6ebf3;
  box-shadow: 0 8px 20px rgba(77, 95, 164, .06);
}
.form-card h1 { margin: 0 0 .4rem; font-size: 1.5rem; color: #1a1a2e; }
.desc { color: #666; margin: 0 0 1.8rem; font-size: .9rem; }

.field { margin-bottom: 1.2rem; }
.field > label { display: block; font-size: .88rem; font-weight: 500; color: #333; margin-bottom: .45rem; }
.required { color: #e53e3e; margin-left: .15rem; }

.range-inline-value {
  margin-left: .45rem;
  font-size: .8rem;
  color: var(--brand-700);
  background: var(--bg-soft-blue);
  border: 1px solid rgba(107, 124, 255, .2);
  border-radius: 999px;
  padding: .12rem .48rem;
  font-weight: 600;
}

.field input:not([type="checkbox"]):not([type="radio"]):not([type="range"]),
select,
textarea {
  width: 100%;
  min-height: 44px;
  padding: .65rem .85rem;
  border: 1.5px solid #d9dfeb;
  border-radius: 8px;
  font-size: .95rem;
  box-sizing: border-box;
  transition: border-color .2s;
  font-family: inherit;
  background: #fff;
  line-height: 1.25;
  appearance: none;
}

/* 时间输入在不同浏览器下默认高度差异较大，单独统一内边距 */
.field input[type="time"] {
  padding-top: .6rem;
  padding-bottom: .6rem;
}

.range-wrap {
  padding: .3rem 0 0;
}

.field input[type="range"] {
  -webkit-appearance: none;
  width: 100%;
  height: 22px;
  border: none;
  padding: 0;
  margin: 0;
  background: transparent;
  outline: none;
  appearance: none;
  cursor: pointer;
}

.field input[type="range"]::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 999px;
  background: linear-gradient(90deg, #dbe3ff, #e8edff);
}

.field input[type="range"]::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--brand-600);
  border: 2px solid #fff;
  margin-top: -5px;
  box-shadow: 0 3px 8px rgba(63, 88, 214, .28);
}

.field input[type="range"]::-moz-range-track {
  height: 4px;
  border: none;
  border-radius: 999px;
  background: linear-gradient(90deg, #dbe3ff, #e8edff);
}

.field input[type="range"]::-moz-range-progress {
  height: 4px;
  border-radius: 999px;
  background: linear-gradient(90deg, #b8c8ff, #d5ddff);
}

.field input[type="range"]::-moz-range-thumb {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--brand-600);
  border: 2px solid #fff;
  box-shadow: 0 3px 8px rgba(63, 88, 214, .28);
}

.range-ticks {
  display: flex;
  justify-content: space-between;
  margin-top: .2rem;
  padding: 0 1px;
  color: var(--text-muted);
  font-size: .78rem;
}

.range-ticks span {
  position: relative;
  min-width: 2ch;
  text-align: center;
  line-height: 1.1;
}

.range-ticks span::before {
  content: '';
  position: absolute;
  left: 50%;
  top: -9px;
  width: 1px;
  height: 6px;
  background: #c8d2ea;
  transform: translateX(-50%);
}

input:focus, select:focus, textarea:focus {
  outline: none;
  border-color: var(--brand-600);
  box-shadow: 0 0 0 3px rgba(75, 104, 242, .13);
}

.input-invalid {
  border-color: #e53e3e !important;
  box-shadow: 0 0 0 3px rgba(229, 62, 62, .12) !important;
}

.field-inline-hint {
  margin: .35rem 0 0;
  color: #5f6b83;
  font-size: .82rem;
}

.field-inline-error {
  margin: .35rem 0 0;
  color: #e53e3e;
  font-size: .82rem;
}

.option-group { display: flex; flex-wrap: wrap; gap: .6rem; }
.option-label {
  display: flex; align-items: center; gap: .35rem;
  font-size: .9rem; cursor: pointer; color: #333;
}

.error-msg { color: #e53e3e; font-size: .85rem; margin-bottom: .8rem; }

.actions { margin-top: 1.5rem; }
.btn-submit {
  width: 100%;
  padding: .8rem;
  background: var(--brand-600);
  color: #fff;
  border: none;
  border-radius: 8px;
  font-size: 1rem;
  cursor: pointer;
  transition: background .2s;
}
.btn-submit:hover:not(:disabled) { background: var(--brand-700); }
.btn-submit:disabled { opacity: .6; cursor: not-allowed; }
</style>
