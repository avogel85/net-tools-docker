<script setup lang="ts">
import type { DnsLookup } from '../dns-lookup/dns-lookup.service'
import type { SnapRecord, Snapshot } from './dns-snapshot.service'
import { NAlert, NButton, NCheckbox, NProgress, NTag } from 'naive-ui'
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiGet } from '@/api/client'
import ResultTable from '@/components/ResultTable.vue'
import { buildQuery } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { downloadText } from '@/utils/download'
import { splitList } from '@/utils/ip'
import { limiter } from '@/utils/limit'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { collect, diff, loadSnapshots, MAX_EXTRA, parseSnapshot, plan, saveSnapshots } from './dns-snapshot.service'

const { t, locale } = useI18n()
const when = (iso: string) => new Date(iso).toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const extra = useQueryParam({ name: 'names', defaultValue: '' })
const resolver = useQueryParam({ name: 'resolver', defaultValue: '' })
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })
const zone = computed(() => domain.value.trim().toLowerCase().replace(/\.$/, ''))

const all = ref<Snapshot[]>(loadSnapshots())
const storageOk = ref(true)
const mine = computed(() => all.value.filter(s => s.domain === zone.value))
function persist() {
  storageOk.value = saveSnapshots(all.value)
}

const before = ref('')
const after = ref('')
const busy = ref(false)
const progress = ref(0)
const total = ref(0)
const latest = shallowRef<Snapshot>()
async function take() {
  if (!zone.value || !validation.value.isValid)
    return
  busy.value = true
  const queries = plan(zone.value, splitList(extra.value))
  total.value = queries.length
  progress.value = 0
  const limit = limiter(3)
  const answers: SnapRecord[] = []
  const errors: Snapshot['errors'] = []
  let label = ''
  await Promise.all(queries.map(q => limit(async () => {
    try {
      const r = await apiGet<DnsLookup>(`/v1/dns/lookup${buildQuery({ name: q.name, type: q.type, resolver: resolver.value || undefined })}`)
      label = r.resolver
      if (r.rcode !== 'NOERROR' && r.rcode !== 'NXDOMAIN')
        errors.push({ ...q, rcode: r.rcode })
      answers.push(...r.answer.filter(a => a.type !== 'RRSIG').map(a => ({ name: a.name, type: a.type, ttl: a.ttl, data: a.data })))
    }
    catch {
      errors.push({ ...q, rcode: 'ERROR' })
    }
    finally {
      progress.value++
    }
  })))
  const snap: Snapshot = { id: `${Date.now()}`, domain: zone.value, takenAt: new Date().toISOString(), resolver: label, records: collect(answers), errors }
  latest.value = snap
  all.value = [snap, ...all.value]
  persist()
  if (mine.value.length >= 2) {
    before.value = mine.value[1]!.id
    after.value = snap.id
  }
  busy.value = false
}

watch(zone, () => {
  before.value = mine.value[1]?.id ?? ''
  after.value = mine.value[0]?.id ?? ''
}, { immediate: true })
const ttl = ref(false)
const serial = ref(false)
const label = (s: Snapshot) => `${when(s.takenAt)} · ${t('tools.dns-snapshot.recordCount', { n: s.records.length })}`
const options = computed(() => mine.value.map(s => ({ value: s.id, label: label(s) })))
const pair = computed(() => {
  const a = mine.value.find(s => s.id === before.value)
  const b = mine.value.find(s => s.id === after.value)
  return a && b && a !== b ? diff(a, b, { ttl: ttl.value, serial: serial.value }) : undefined
})

function remove(id: string) {
  all.value = all.value.filter(s => s.id !== id)
  persist()
  if (latest.value?.id === id)
    latest.value = undefined
}
const fileInput = ref<HTMLInputElement>()
const importError = ref(false)
async function importFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f)
    return
  const s = parseSnapshot(await f.text())
  importError.value = !s
  if (s) {
    all.value = [s, ...all.value.filter(x => x.id !== s.id)]
    persist()
    domain.value = s.domain
  }
  ;(e.target as HTMLInputElement).value = ''
}
const shown = computed(() => latest.value ?? mine.value[0])
const recordColumns = computed(() => [
  { key: 'name', title: t('tools.dns-snapshot.name'), mono: true, width: 220 },
  { key: 'type', title: t('tools.dns-snapshot.type'), width: 80 },
  { key: 'ttl', title: 'TTL', width: 70 },
  { key: 'data', title: t('tools.dns-snapshot.data'), mono: true, width: 420 },
])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="take">
      <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" :validation="validation" monospace test-id="domain" />
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
        <CInputText v-model:value="extra" :label="t('tools.dns-snapshot.extra', { max: MAX_EXTRA })" placeholder="shop, api, selector1._domainkey" monospace test-id="names" />
        <CInputText v-model:value="resolver" :label="t('tools.dns-snapshot.resolver')" placeholder="system" monospace test-id="resolver" />
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <NButton type="primary" attr-type="submit" :loading="busy" :disabled="!domain || !validation.isValid" data-testid="run">
          {{ t('tools.dns-snapshot.take') }}
        </NButton>
        <NButton secondary @click="fileInput?.click()">
          {{ t('tools.dns-snapshot.import') }}
        </NButton>
        <input ref="fileInput" type="file" accept="application/json,.json" class="hidden" :aria-label="t('tools.dns-snapshot.import')" @change="importFile">
      </div>
      <NProgress v-if="busy" type="line" class="mt-3 max-w-md" :percentage="total ? Math.round(progress / total * 100) : 0" :show-indicator="false" :aria-label="t('tools.dns-snapshot.progress')" />
    </form>
  </CCard>
  <NAlert v-if="!storageOk" type="warning" class="mb-4">
    {{ t('tools.dns-snapshot.storageFailed') }}
  </NAlert>
  <NAlert v-if="importError" type="error" class="mb-4">
    {{ t('tools.dns-snapshot.importError') }}
  </NAlert>

  <CCard v-if="mine.length >= 2" :title="t('tools.dns-snapshot.compare')" data-testid="snapshot-compare">
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CSelect v-model:value="before" :label="t('tools.dns-snapshot.before')" :options="options" />
      <CSelect v-model:value="after" :label="t('tools.dns-snapshot.after')" :options="options" />
    </div>
    <div class="mb-3 flex flex-wrap gap-x-6">
      <NCheckbox v-model:checked="ttl">
        {{ t('tools.dns-snapshot.compareTtl') }}
      </NCheckbox>
      <NCheckbox v-model:checked="serial">
        {{ t('tools.dns-snapshot.compareSerial') }}
      </NCheckbox>
    </div>
    <template v-if="pair">
      <NAlert v-if="!pair.added.length && !pair.removed.length && !pair.changed.length" type="success" data-testid="no-changes">
        {{ t('tools.dns-snapshot.noChanges', { n: pair.same }) }}
      </NAlert>
      <ul v-else class="m-0 list-none p-0 text-sm" data-testid="diff">
        <li v-for="c in pair.added" :key="`a${c.name}${c.type}`" class="py-1">
          <NTag size="small" type="success" :bordered="false">
            {{ t('tools.dns-snapshot.added') }}
          </NTag>
          <span class="ml-2 font-mono">{{ c.name }} {{ c.type }}</span>
          <div class="break-all pl-4 font-mono">
            + {{ c.after.join(' | ') }}
          </div>
        </li>
        <li v-for="c in pair.removed" :key="`r${c.name}${c.type}`" class="py-1">
          <NTag size="small" type="error" :bordered="false">
            {{ t('tools.dns-snapshot.removed') }}
          </NTag>
          <span class="ml-2 font-mono">{{ c.name }} {{ c.type }}</span>
          <div class="break-all pl-4 font-mono">
            − {{ c.before.join(' | ') }}
          </div>
        </li>
        <li v-for="c in pair.changed" :key="`c${c.name}${c.type}`" class="py-1">
          <NTag size="small" type="warning" :bordered="false">
            {{ t('tools.dns-snapshot.changed') }}
          </NTag>
          <span class="ml-2 font-mono">{{ c.name }} {{ c.type }}</span>
          <div class="break-all pl-4 font-mono">
            − {{ c.before.join(' | ') }}
          </div>
          <div class="break-all pl-4 font-mono">
            + {{ c.after.join(' | ') }}
          </div>
        </li>
      </ul>
    </template>
  </CCard>

  <CCard v-if="shown" :title="t('tools.dns-snapshot.snapshotOf', { date: when(shown.takenAt) })" data-testid="result">
    <p class="mt-0 text-sm">
      {{ t('tools.dns-snapshot.summary', { n: shown.records.length, resolver: shown.resolver || '—' }) }}
    </p>
    <NAlert v-if="shown.errors.length" type="warning" class="mb-3">
      {{ t('tools.dns-snapshot.errors', { list: shown.errors.map(e => `${e.name} ${e.type} (${e.rcode})`).join(', ') }) }}
    </NAlert>
    <ResultTable :columns="recordColumns" :rows="shown.records" :actions="false" :label="t('tools.dns-snapshot.records')" />
  </CCard>

  <CCard v-if="mine.length" :title="t('tools.dns-snapshot.saved', { n: mine.length })" data-testid="saved">
    <p class="mt-0 text-sm">
      {{ t('tools.dns-snapshot.savedHint') }}
    </p>
    <div v-for="s in mine" :key="s.id" class="flex flex-wrap items-center gap-2 py-1 text-sm">
      <span class="min-w-60 flex-1">{{ label(s) }}</span>
      <NButton size="tiny" secondary @click="downloadText(`dns-snapshot-${s.domain}-${s.takenAt.slice(0, 10)}.json`, JSON.stringify(s, null, 2))">
        {{ t('tools.dns-snapshot.export') }}
      </NButton>
      <NButton size="tiny" secondary @click="remove(s.id)">
        {{ t('tools.dns-snapshot.delete') }}
      </NButton>
    </div>
  </CCard>
</template>
