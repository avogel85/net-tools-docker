<script setup lang="ts">
import type { Check, Store } from './expiry-watchlist.service'
import type { components } from '@/api/schema'
import { NAlert, NButton, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiGet, apiPost } from '@/api/client'
import ResultTable from '@/components/ResultTable.vue'
import { buildQuery } from '@/composables/useApi'
import CCard from '@/ui/c-card.vue'
import CSelect from '@/ui/c-select.vue'
import { downloadText } from '@/utils/download'
import { limiter } from '@/utils/limit'
import { domainsToCheck, level, load, MAX_ENTRIES, mergeEntries, parseEntries, rows, save, upsertChecks } from './expiry-watchlist.service'

type TlsExpiry = components['schemas']['TlsExpiry']
type DomainExpiry = components['schemas']['DomainExpiry']

const { t, locale } = useI18n()
const store = ref<Store>(load())
const storageOk = ref(true)
function persist() {
  storageOk.value = save(store.value)
}

const input = ref('')
const invalid = ref<string[]>([])
function add() {
  const r = parseEntries(input.value)
  invalid.value = r.invalid
  store.value = { ...store.value, entries: mergeEntries(store.value.entries, r.entries) }
  input.value = r.invalid.join('\n')
  persist()
  void check()
}
function remove(target: string) {
  store.value = { ...store.value, entries: store.value.entries.filter(e => e.target !== target) }
  persist()
}

const busy = ref(false)
const pending = ref(0)
async function check() {
  if (busy.value || !store.value.entries.length)
    return
  busy.value = true
  const now = new Date().toISOString()
  const fresh: Check[] = []
  const certs = store.value.entries.filter(e => e.kinds.includes('cert')).map(e => e.target)
  const domains = domainsToCheck(store.value.entries, store.value.checks)
  pending.value = domains.length
  const tasks: Promise<void>[] = []
  if (certs.length) {
    tasks.push(apiPost<TlsExpiry>('/v1/tls/expiry', { targets: certs }).then((r) => {
      for (const row of r.rows)
        fresh.push({ target: row.target, kind: 'cert', checkedAt: now, expires: row.notAfter, daysLeft: row.daysLeft, by: row.issuer, error: row.error })
    }).catch((e: Error) => {
      for (const c of certs)
        fresh.push({ target: c, kind: 'cert', checkedAt: now, expires: '', daysLeft: null, by: '', error: e.message })
    }))
  }
  const limit = limiter(2)
  for (const d of domains) {
    tasks.push(limit(() => apiGet<DomainExpiry>(`/v1/domain/expiry${buildQuery({ domain: d })}`)).then((r) => {
      fresh.push({ target: d, kind: 'domain', checkedAt: now, expires: r.expires, daysLeft: r.daysLeft, by: r.registrar, error: '' })
    }).catch((e: Error) => {
      fresh.push({ target: d, kind: 'domain', checkedAt: now, expires: '', daysLeft: null, by: '', error: e.message })
    }).finally(() => {
      pending.value--
    }))
  }
  await Promise.all(tasks)
  store.value = { ...store.value, checks: upsertChecks(store.value.checks, fresh) }
  persist()
  busy.value = false
}
onMounted(() => {
  void check()
})

const warnOptions = [7, 14, 30, 60].map(v => ({ value: v, label: `${v}` }))
const warnDays = computed({
  get: () => store.value.warnDays,
  set: (v: number) => {
    store.value = { ...store.value, warnDays: v }
    persist()
  },
})
const levelType = { expired: 'error', soon: 'warning', ok: 'success', unknown: 'default' } as const
const date = (iso: string) => (iso ? new Date(iso).toLocaleDateString(locale.value) : '—')
const tableRows = computed(() => rows(store.value).map(c => ({
  level: t(`tools.expiry-watchlist.levels.${level(c, store.value.warnDays)}`),
  target: c.target,
  kind: t(`tools.expiry-watchlist.kinds.${c.kind}`),
  expires: date(c.expires),
  days: c.daysLeft ?? '—',
  by: c.error || c.by || '—',
  checked: date(c.checkedAt),
})))
const counts = computed(() => {
  const out = { expired: 0, soon: 0, ok: 0, unknown: 0 }
  for (const c of rows(store.value)) out[level(c, store.value.warnDays)]++
  return out
})
const columns = computed(() => [
  { key: 'level', title: t('tools.expiry-watchlist.status'), width: 120 },
  { key: 'target', title: t('tools.expiry-watchlist.target'), mono: true, width: 220 },
  { key: 'kind', title: t('tools.expiry-watchlist.kind'), width: 120 },
  { key: 'expires', title: t('tools.expiry-watchlist.expires'), width: 120 },
  { key: 'days', title: t('tools.expiry-watchlist.days'), width: 90 },
  { key: 'by', title: t('tools.expiry-watchlist.by'), width: 260 },
  { key: 'checked', title: t('tools.expiry-watchlist.checked'), width: 110 },
])
const fileInput = ref<HTMLInputElement>()
async function importFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (f) {
    try {
      const data = JSON.parse(await f.text()) as Partial<Store>
      if (Array.isArray(data.entries)) {
        store.value = { ...store.value, entries: mergeEntries(store.value.entries, data.entries) }
        persist()
        void check()
      }
    }
    catch {}
  }
  ;(e.target as HTMLInputElement).value = ''
}
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.expiry-watchlist.add', { max: MAX_ENTRIES })" :validation-status="invalid.length ? 'error' : undefined" :feedback="invalid.length ? t('tools.expiry-watchlist.invalid', { list: invalid.join(', ') }) : undefined">
      <NInput v-model:value="input" type="textarea" :autosize="{ minRows: 2, maxRows: 8 }" class="font-mono" placeholder="example.com&#10;mail.example.com:465&#10;domain:example.org" :input-props="{ 'aria-label': t('tools.expiry-watchlist.add', { max: MAX_ENTRIES }), 'spellcheck': false }" data-testid="input" />
    </NFormItem>
    <p class="mb-3 mt-0 text-sm">
      {{ t('tools.expiry-watchlist.hint') }}
    </p>
    <div class="flex flex-wrap items-end gap-3">
      <NButton type="primary" :disabled="!input.trim()" data-testid="add" @click="add">
        {{ t('tools.expiry-watchlist.addButton') }}
      </NButton>
      <NButton secondary :loading="busy" :disabled="!store.entries.length" data-testid="run" @click="check">
        {{ t('tools.expiry-watchlist.check') }}
      </NButton>
      <div class="w-40">
        <CSelect v-model:value="warnDays" :label="t('tools.expiry-watchlist.warnDays')" :options="warnOptions" />
      </div>
    </div>
  </CCard>
  <NAlert v-if="!storageOk" type="warning" class="mb-4">
    {{ t('tools.expiry-watchlist.storageFailed') }}
  </NAlert>
  <NAlert v-if="busy && pending" type="info" class="mb-4">
    {{ t('tools.expiry-watchlist.domainsPending', { n: pending }) }}
  </NAlert>
  <CCard v-if="tableRows.length" :title="t('tools.expiry-watchlist.results')" data-testid="result">
    <div class="mb-3 flex flex-wrap gap-2">
      <NTag v-for="(n, k) in counts" :key="k" size="small" :type="levelType[k]" :bordered="false">
        {{ t(`tools.expiry-watchlist.levels.${k}`) }}: {{ n }}
      </NTag>
    </div>
    <ResultTable :columns="columns" :rows="tableRows" filename="expiry-watchlist.json" :label="t('tools.expiry-watchlist.results')" />
  </CCard>
  <CCard v-if="store.entries.length" :title="t('tools.expiry-watchlist.entries', { n: store.entries.length })" data-testid="entries">
    <div v-for="e in store.entries" :key="e.target" class="flex flex-wrap items-center gap-2 py-0.5 text-sm">
      <span class="min-w-48 flex-1 break-all font-mono">{{ e.target }}</span>
      <NTag v-for="k in e.kinds" :key="k" size="tiny" :bordered="false">
        {{ t(`tools.expiry-watchlist.kinds.${k}`) }}
      </NTag>
      <NButton size="tiny" secondary @click="remove(e.target)">
        {{ t('tools.expiry-watchlist.remove') }}
      </NButton>
    </div>
    <div class="mt-3 flex flex-wrap gap-2">
      <NButton size="small" secondary @click="downloadText('expiry-watchlist.json', JSON.stringify({ entries: store.entries }, null, 2))">
        {{ t('tools.expiry-watchlist.export') }}
      </NButton>
      <NButton size="small" secondary @click="fileInput?.click()">
        {{ t('tools.expiry-watchlist.import') }}
      </NButton>
      <input ref="fileInput" type="file" accept="application/json,.json" class="hidden" :aria-label="t('tools.expiry-watchlist.import')" @change="importFile">
    </div>
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.expiry-watchlist.storageHint') }}
    </p>
  </CCard>
</template>
