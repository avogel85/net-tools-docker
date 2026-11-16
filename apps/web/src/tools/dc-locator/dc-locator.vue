<script setup lang="ts">
import type { DcLocator } from './dc-locator.service'
import { NAlert, NButton } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import { useCapabilitiesStore } from '@/stores/capabilities.store'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { defaultResolver, lookupRows } from './dc-locator.service'

const { t } = useI18n()
const caps = useCapabilitiesStore()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const site = useQueryParam({ name: 'site', defaultValue: '' })
const resolver = useQueryParam({ name: 'resolver', defaultValue: '' })
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })
const siteOk = computed(() => /^[\w-]{0,63}$/.test(site.value))
const { data, error, loading, run } = useApi<DcLocator>('/v1/dns/dc-locator')
function submit() {
  if (domain.value && validation.value.isValid && siteOk.value)
    void run({ domain: domain.value, site: site.value || undefined, resolver: resolver.value || defaultResolver(caps.capabilities?.mode.dnsUpstream) })
}
onMounted(() => {
  if (domain.value)
    submit()
})
const rows = computed(() => (data.value ? lookupRows(data.value).map(r => ({ ...r, status: t(`tools.dc-locator.status.${r.status}`) })) : []))
const dcRows = computed(() => (data.value?.dcs ?? []).map(d => ({ name: d.name, addresses: d.addresses.join(', ') || '—', services: d.services.map(s => t(`tools.dc-locator.services.${s}`)).join(', ') })))
const lookupColumns = computed(() => [
  { key: 'name', title: t('tools.dc-locator.record'), mono: true, width: 330 },
  { key: 'status', title: t('tools.dc-locator.statusTitle'), width: 120 },
  { key: 'targets', title: t('tools.dc-locator.targets'), mono: true, width: 300 },
  { key: 'port', title: t('tools.dc-locator.port'), width: 70 },
])
const dcColumns = computed(() => [
  { key: 'name', title: t('tools.dc-locator.dc'), mono: true, width: 220 },
  { key: 'addresses', title: t('tools.dc-locator.addresses'), mono: true, width: 220 },
  { key: 'services', title: t('tools.dc-locator.servicesTitle'), width: 360 },
])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="domain" :label="t('tools.dc-locator.domain')" placeholder="corp.example.com" :validation="validation" monospace test-id="domain" />
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
        <CInputText v-model:value="site" :label="t('tools.dc-locator.site')" placeholder="Default-First-Site-Name" monospace test-id="site" :validation="siteOk ? undefined : { isValid: false, status: 'error', message: t('tools.dc-locator.invalidSite') }" />
        <CInputText v-model:value="resolver" :label="t('tools.dc-locator.resolver')" :placeholder="defaultResolver(caps.capabilities?.mode.dnsUpstream)" monospace test-id="resolver" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.dc-locator.resolverHint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!domain || !validation.isValid || !siteOk" data-testid="run">
        {{ t('tools.dc-locator.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <div class="mb-4 flex flex-col gap-2" data-testid="findings">
      <NAlert v-for="p in data.problems" :key="p" type="error">
        {{ t(`tools.dc-locator.findings.${p}`) }}
      </NAlert>
      <NAlert v-for="w in data.warnings" :key="w" type="warning">
        {{ t(`tools.dc-locator.findings.${w}`) }}
      </NAlert>
      <NAlert v-if="!data.problems.length && !data.warnings.length" type="success">
        {{ t('tools.dc-locator.ok', { n: data.dcs.length }) }}
      </NAlert>
    </div>
    <CCard v-if="dcRows.length" :title="t('tools.dc-locator.dcsTitle')" data-testid="dcs">
      <ResultTable :columns="dcColumns" :rows="dcRows" filename="domain-controllers.json" :label="t('tools.dc-locator.dcsTitle')" />
    </CCard>
    <CCard :title="t('tools.dc-locator.lookupsTitle', { resolver: data.resolver })" data-testid="result">
      <ResultTable :columns="lookupColumns" :rows="rows" filename="dc-locator.json" :label="t('tools.dc-locator.lookupsTitle', { resolver: data.resolver })" />
    </CCard>
  </template>
</template>
