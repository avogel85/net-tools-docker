<script setup lang="ts">
import type { DnsLookup } from '../dns-lookup/dns-lookup.service'
import { NAlert, NButton, NFormItem, NInput, NInputNumber } from 'naive-ui'
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiGet } from '@/api/client'
import ResultTable from '@/components/ResultTable.vue'
import { buildQuery } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { MAX_ITERATIONS, nsec3Hash, paramWarnings, parseNsec3Param, parseSalt } from './nsec3-hash.service'

const { t } = useI18n()
const zone = useQueryParam({ name: 'domain', defaultValue: '' })
const names = useQueryParam({ name: 'names', defaultValue: '' })
const salt = useQueryParam({ name: 'salt', defaultValue: '' })
const iterations = useQueryParam({ name: 'iterations', defaultValue: 0 })

const saltBytes = computed(() => parseSalt(salt.value))
const itersOk = computed(() => Number.isInteger(iterations.value) && iterations.value >= 0 && iterations.value <= MAX_ITERATIONS)
const itersModel = computed({
  get: () => iterations.value,
  set: (v: number | null) => {
    iterations.value = v ?? 0
  },
})
const zoneName = computed(() => zone.value.trim().toLowerCase().replace(/\.$/, ''))
/** Names to hash: absolute as given, relative ones (no trailing dot, not ending in the zone) get the zone appended. */
const list = computed(() => {
  const out = names.value.split(/[\s,]+/).map(n => n.trim().toLowerCase()).filter(Boolean)
  const all = out.length ? out : (zoneName.value ? [zoneName.value] : [])
  return [...new Set(all.map((n) => {
    if (n.endsWith('.') || !zoneName.value || n === zoneName.value || n.endsWith(`.${zoneName.value}`))
      return n.replace(/\.$/, '')
    return `${n}.${zoneName.value}`
  }))].slice(0, 100)
})

const rows = shallowRef<{ name: string, hash: string, owner: string }[]>([])
let seq = 0
watch([list, saltBytes, iterations], async () => {
  const id = ++seq
  if (!saltBytes.value || !itersOk.value) {
    rows.value = []
    return
  }
  const out = []
  for (const name of list.value) {
    const hash = await nsec3Hash(name, saltBytes.value, iterations.value)
    out.push({ name, hash, owner: zoneName.value ? `${hash}.${zoneName.value}` : hash })
  }
  if (id === seq)
    rows.value = out
}, { immediate: true })
const warnings = computed(() => (saltBytes.value && itersOk.value ? paramWarnings(iterations.value, saltBytes.value) : []))

const loading = ref(false)
const loadError = ref('')
const via = ref('')
async function loadParams() {
  if (!isValidQueryName(zoneName.value))
    return
  loading.value = true
  loadError.value = ''
  try {
    const r = await apiGet<DnsLookup>(`/v1/dns/lookup${buildQuery({ name: zoneName.value, type: 'NSEC3PARAM', dnssec: 'true' })}`)
    let data = r.answer.filter(a => a.type === 'NSEC3PARAM').map(a => a.data)
    // A filtering local resolver (dnsmasq/Pi-hole) can drop NSEC3PARAM; the API then attaches the
    // reference resolver's answer.
    via.value = ''
    if (!data.length && r.filtered?.referenceData.length) {
      data = r.filtered.referenceData
      via.value = r.filtered.reference
    }
    const p = data.map(parseNsec3Param).find(Boolean)
    if (!p) {
      loadError.value = t('tools.nsec3-hash.noParam', { zone: zoneName.value, rcode: r.rcode })
      return
    }
    salt.value = p.salt === '-' ? '' : p.salt.toLowerCase()
    iterations.value = p.iterations
  }
  catch {
    loadError.value = t('tools.nsec3-hash.loadFailed')
  }
  finally {
    loading.value = false
  }
}
const columns = computed(() => [
  { key: 'name', title: t('tools.nsec3-hash.name'), mono: true, width: 220 },
  { key: 'hash', title: t('tools.nsec3-hash.hash'), mono: true, width: 300 },
  { key: 'owner', title: t('tools.nsec3-hash.owner'), mono: true, width: 400 },
])
</script>

<template>
  <CCard>
    <CInputText v-model:value="zone" :label="t('tools.nsec3-hash.zone')" placeholder="example.com" monospace test-id="domain" />
    <NButton secondary class="mb-4" :loading="loading" :disabled="!isValidQueryName(zoneName)" data-testid="load" @click="loadParams">
      {{ t('tools.nsec3-hash.load') }}
    </NButton>
    <NAlert v-if="loadError" type="warning" class="mb-4" data-testid="load-error">
      {{ loadError }}
    </NAlert>
    <NAlert v-if="via" type="info" class="mb-4" data-testid="via">
      {{ t('tools.nsec3-hash.via', { resolver: via }) }}
    </NAlert>
    <NFormItem :label="t('tools.nsec3-hash.names')">
      <NInput v-model:value="names" type="textarea" :autosize="{ minRows: 2, maxRows: 10 }" class="font-mono" placeholder="www&#10;mail&#10;*.example.com." :input-props="{ 'aria-label': t('tools.nsec3-hash.names'), 'spellcheck': false }" data-testid="names" />
    </NFormItem>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="salt" :label="t('tools.nsec3-hash.salt')" placeholder="-" monospace test-id="salt" :validation="saltBytes ? undefined : { isValid: false, status: 'error', message: t('tools.nsec3-hash.invalidSalt') }" />
      <NFormItem :label="t('tools.nsec3-hash.iterations')" :validation-status="itersOk ? undefined : 'error'" :feedback="itersOk ? undefined : t('tools.nsec3-hash.invalidIterations', { max: MAX_ITERATIONS })">
        <NInputNumber v-model:value="itersModel" :min="0" :max="MAX_ITERATIONS" :show-button="false" :input-props="{ 'aria-label': t('tools.nsec3-hash.iterations') }" data-testid="iterations" />
      </NFormItem>
    </div>
  </CCard>
  <div v-if="warnings.length" class="mb-4 flex flex-col gap-2">
    <NAlert v-for="w in warnings" :key="w" type="warning">
      {{ t(`tools.nsec3-hash.findings.${w}`) }}
    </NAlert>
  </div>
  <CCard v-if="rows.length" :title="t('common.result')" data-testid="result">
    <ResultTable :columns="columns" :rows="rows" filename="nsec3.json" :label="t('common.result')" />
  </CCard>
</template>
