<script setup lang="ts">
import type { DnsLookup } from '../dns-lookup/dns-lookup.service'
import type { Comparison, Dnskey } from './ds-dnskey.service'
import { NAlert, NButton, NFormItem, NInput, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError as ApiErr, apiGet } from '@/api/client'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { buildQuery } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { ALGORITHMS, compare, computeDs, DIGESTS, dsText, keyRole, parseDnskeys, parseDs } from './ds-dnskey.service'

const { t } = useI18n()
const mode = useQueryParam({ name: 'mode', defaultValue: 'domain' })
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const owner = ref('')
const pasted = ref('')
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })

interface KeyRow { tag: number, role: string, algorithm: string, ds256: string, ds384: string }
const keyRows = shallowRef<KeyRow[]>([])
const comparison = shallowRef<Comparison>()
const fetched = shallowRef<{ owner: string, keys: Dnskey[] }>()
const loading = ref(false)
const error = shallowRef<ApiErr>()
const parseError = ref(false)

const alg = (n: number) => `${n} ${ALGORITHMS[n] ?? ''}`.trim()
async function rowsFor(name: string, keys: Dnskey[]): Promise<KeyRow[]> {
  return Promise.all(keys.map(async k => ({
    tag: k.keyTag,
    role: t(`tools.ds-dnskey.roles.${keyRole(k)}`),
    algorithm: alg(k.algorithm),
    ds256: keyRole(k) === 'zsk' ? '—' : dsText(await computeDs(name, k, 2)),
    ds384: keyRole(k) === 'zsk' ? '—' : dsText(await computeDs(name, k, 4)),
  })))
}

// DO bit set: some resolvers (dnsmasq behind Pi-hole) answer NOTIMP to DNSKEY queries without it.
async function lookup(type: 'DNSKEY' | 'DS', name: string): Promise<string[]> {
  const r = await apiGet<DnsLookup>(`/v1/dns/lookup${buildQuery({ name, type, dnssec: 'true' })}`)
  if (r.rcode !== 'NOERROR' && r.rcode !== 'NXDOMAIN')
    throw new ResolverError(type, r.rcode, r.resolver)
  return r.answer.filter(a => a.type === type).map(a => a.data)
}
class ResolverError extends Error {
  constructor(public type: string, public rcode: string, public resolver: string) {
    super(`${type} ${rcode}`)
  }
}
const resolverError = shallowRef<ResolverError>()
async function check() {
  if (!domain.value || !validation.value.isValid)
    return
  loading.value = true
  error.value = undefined
  resolverError.value = undefined
  comparison.value = undefined
  try {
    const name = domain.value.trim().toLowerCase().replace(/\.$/, '')
    const [keyData, dsData] = await Promise.all([lookup('DNSKEY', name), lookup('DS', name)])
    const keys = (parseDnskeys(keyData.join('\n')) ?? []).map(k => k.key)
    fetched.value = { owner: name, keys }
    comparison.value = await compare(name, keys, parseDs(dsData.join('\n')))
    keyRows.value = await rowsFor(name, keys)
  }
  catch (e) {
    if (e instanceof ResolverError)
      resolverError.value = e
    else
      error.value = e instanceof ApiErr ? e : new ApiErr('NETWORK_ERROR', String(e))
  }
  finally {
    loading.value = false
  }
}

async function compute() {
  const parsed = parseDnskeys(pasted.value)
  parseError.value = !parsed
  if (!parsed) {
    keyRows.value = []
    return
  }
  const name = owner.value.trim() || parsed[0]?.owner || ''
  owner.value = name
  keyRows.value = name ? await rowsFor(name, parsed.map(p => p.key)) : []
}
watch(mode, () => {
  keyRows.value = []
  comparison.value = undefined
})

const dsRows = computed(() => (comparison.value?.ds ?? []).map(d => ({
  tag: d.ds.keyTag,
  algorithm: alg(d.ds.algorithm),
  digest: DIGESTS[d.ds.digestType]?.name ?? String(d.ds.digestType),
  status: t(`tools.ds-dnskey.status.${d.match ? 'match' : d.key ? 'mismatch' : 'orphan'}`),
  value: d.ds.digest,
})))
const verdict = computed(() => {
  const c = comparison.value
  if (!c || !fetched.value)
    return undefined
  if (!fetched.value.keys.length && !c.ds.length)
    return { type: 'info' as const, text: t('tools.ds-dnskey.verdict.unsigned') }
  if (!c.ds.length)
    return { type: 'warning' as const, text: t('tools.ds-dnskey.verdict.noDs') }
  if (!fetched.value.keys.length)
    return { type: 'error' as const, text: t('tools.ds-dnskey.verdict.noKeys') }
  if (!c.ds.some(d => d.match))
    return { type: 'error' as const, text: t('tools.ds-dnskey.verdict.broken') }
  if (c.ds.some(d => !d.match) || c.unlinked.length)
    return { type: 'warning' as const, text: t('tools.ds-dnskey.verdict.rollover') }
  return { type: 'success' as const, text: t('tools.ds-dnskey.verdict.ok') }
})
const keyColumns = computed(() => [
  { key: 'tag', title: t('tools.ds-dnskey.keyTag'), width: 90 },
  { key: 'role', title: t('tools.ds-dnskey.role'), width: 90 },
  { key: 'algorithm', title: t('tools.ds-dnskey.algorithm'), width: 170 },
  { key: 'ds256', title: 'DS (SHA-256)', mono: true, width: 320 },
  { key: 'ds384', title: 'DS (SHA-384)', mono: true, width: 380 },
])
const dsColumns = computed(() => [
  { key: 'tag', title: t('tools.ds-dnskey.keyTag'), width: 90 },
  { key: 'algorithm', title: t('tools.ds-dnskey.algorithm'), width: 170 },
  { key: 'digest', title: t('tools.ds-dnskey.digestType'), width: 100 },
  { key: 'status', title: t('tools.ds-dnskey.statusTitle'), width: 170 },
  { key: 'value', title: t('tools.ds-dnskey.digest'), mono: true, width: 380 },
])
</script>

<template>
  <CCard>
    <NRadioGroup v-model:value="mode" name="mode" class="mb-4">
      <NRadioButton value="domain" data-testid="mode-domain">
        {{ t('tools.ds-dnskey.modes.domain') }}
      </NRadioButton>
      <NRadioButton value="paste" data-testid="mode-paste">
        {{ t('tools.ds-dnskey.modes.paste') }}
      </NRadioButton>
    </NRadioGroup>
    <form v-if="mode === 'domain'" class="contents" @submit.prevent="check">
      <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" :validation="validation" monospace test-id="domain" />
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!domain || !validation.isValid" data-testid="run">
        {{ t('tools.ds-dnskey.check') }}
      </NButton>
    </form>
    <form v-else class="contents" @submit.prevent="compute">
      <CInputText v-model:value="owner" :label="t('tools.ds-dnskey.owner')" placeholder="example.com" monospace test-id="owner" />
      <NFormItem :label="t('tools.ds-dnskey.dnskeys')" :validation-status="parseError ? 'error' : undefined" :feedback="parseError ? t('tools.ds-dnskey.parseError') : undefined">
        <NInput v-model:value="pasted" type="textarea" :autosize="{ minRows: 3, maxRows: 12 }" class="font-mono" placeholder="example.com. 3600 IN DNSKEY 257 3 13 mdsswUyr3DPW…" :input-props="{ 'aria-label': t('tools.ds-dnskey.dnskeys'), 'spellcheck': false }" data-testid="dnskeys" />
      </NFormItem>
      <NButton type="primary" attr-type="submit" :disabled="!pasted.trim()" data-testid="compute">
        {{ t('tools.ds-dnskey.compute') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <NAlert v-if="resolverError" type="error" class="mb-4" data-testid="resolver-error">
    {{ t('tools.ds-dnskey.resolverError', { type: resolverError.type, rcode: resolverError.rcode, resolver: resolverError.resolver }) }}
  </NAlert>
  <NAlert v-if="mode === 'domain' && verdict" :type="verdict.type" class="mb-4" data-testid="verdict">
    {{ verdict.text }}
  </NAlert>
  <CCard v-if="mode === 'domain' && dsRows.length" :title="t('tools.ds-dnskey.published')" data-testid="ds">
    <ResultTable :columns="dsColumns" :rows="dsRows" filename="ds.json" :label="t('tools.ds-dnskey.published')" />
  </CCard>
  <CCard v-if="keyRows.length" :title="t('tools.ds-dnskey.keys')" data-testid="keys">
    <p class="mt-0 text-sm">
      {{ t('tools.ds-dnskey.keysHint') }}
    </p>
    <ResultTable :columns="keyColumns" :rows="keyRows" filename="dnskey.json" :label="t('tools.ds-dnskey.keys')" />
  </CCard>
</template>
