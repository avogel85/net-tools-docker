<script setup lang="ts">
import type { UdpProbe } from './udp-probe.service'
import { NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { SERVICES, sortResults, STATE_TYPE } from './udp-probe.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const service = useQueryParam({ name: 'service', defaultValue: 'all' })
const port = useQueryParam({ name: 'port', defaultValue: '' })
const portOk = computed(() => port.value === '' ? service.value !== 'custom' : /^\d{1,5}$/.test(port.value) && Number(port.value) >= 1 && Number(port.value) <= 65535)
const { data, error, loading, run } = useApi<UdpProbe>('/v1/udp/probe')
const submit = () => host.value.trim() && portOk.value && run({ host: host.value.trim(), service: service.value, port: port.value || undefined })
onMounted(() => {
  if (host.value)
    void submit()
})
const options = computed(() => SERVICES.map(s => ({ value: s, label: t(`tools.udp-probe.services.${s}`) })))
const rows = computed(() => (data.value ? sortResults(data.value.results) : []).map(r => ({
  service: t(`tools.udp-probe.services.${r.service}`),
  port: r.port,
  state: t(`tools.udp-probe.states.${r.state}`),
  detail: r.detail || '—',
  rtt: r.rttMs ? `${r.rttMs.toFixed(1)} ms` : '—',
  raw: r.state,
})))
const columns = computed(() => [
  { key: 'service', title: t('tools.udp-probe.service'), width: 160 },
  { key: 'port', title: t('tools.udp-probe.port'), width: 70 },
  { key: 'state', title: t('tools.udp-probe.state'), width: 150 },
  { key: 'detail', title: t('tools.udp-probe.detail'), width: 380 },
  { key: 'rtt', title: 'RTT', width: 90 },
])
const openCount = computed(() => data.value?.results.filter(r => r.state === 'open').length ?? 0)
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_14rem_8rem]">
        <CInputText v-model:value="host" :label="t('tools.udp-probe.host')" placeholder="ntp.example.com" monospace test-id="host" />
        <CSelect v-model:value="service" :label="t('tools.udp-probe.service')" :options="options" />
        <CInputText v-model:value="port" :label="t('tools.udp-probe.port')" :placeholder="service === 'custom' ? '1–65535' : t('tools.udp-probe.default')" monospace test-id="port" :validation="portOk ? undefined : { isValid: false, status: 'error', message: '1–65535' }" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.udp-probe.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!host.trim() || !portOk" data-testid="run">
        {{ t('tools.udp-probe.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="t('tools.udp-probe.results', { host: data.host, address: data.address })" data-testid="result">
    <div class="mb-3 flex flex-wrap gap-2">
      <NTag :type="openCount ? 'success' : 'default'" :bordered="false">
        {{ t('tools.udp-probe.openCount', { n: openCount, total: data.results.length }) }}
      </NTag>
      <NTag v-for="s in (['open|filtered', 'closed'] as const)" :key="s" size="small" :type="STATE_TYPE[s]" :bordered="false">
        {{ t(`tools.udp-probe.states.${s}`) }}: {{ data.results.filter(r => r.state === s).length }}
      </NTag>
    </div>
    <ResultTable :columns="columns" :rows="rows" filename="udp-probe.json" :label="t('tools.udp-probe.service')" />
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.udp-probe.statesHint') }}
    </p>
  </CCard>
</template>
