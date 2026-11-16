<script setup lang="ts">
import type { SipCheck } from './sip-check.service'
import { NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { probeState } from './sip-check.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const transport = useQueryParam({ name: 'transport', defaultValue: 'all' })
const { data, error, loading, run } = useApi<SipCheck>('/v1/sip/check')
const submit = () => host.value.trim() && run({ host: host.value.trim(), transport: transport.value })
onMounted(() => {
  if (host.value)
    void submit()
})
const transports = computed(() => ['all', 'udp', 'tcp', 'tls'].map(v => ({ value: v, label: t(`tools.sip-check.transports.${v}`) })))
const stateType = { ok: 'success', answered: 'info', down: 'error' } as const
const srvRows = computed(() => (data.value?.srv ?? []).map(s => ({ ...s, target: `${s.target}:${s.port}` })))
const srvColumns = computed(() => [
  { key: 'service', title: t('tools.sip-check.service'), mono: true, width: 120 },
  { key: 'priority', title: t('tools.sip-check.priority'), width: 80 },
  { key: 'weight', title: t('tools.sip-check.weight'), width: 80 },
  { key: 'target', title: t('tools.sip-check.target'), mono: true, width: 280 },
])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_12rem]">
        <CInputText v-model:value="host" :label="t('tools.sip-check.host')" placeholder="example.com, pbx.example.com:5060" monospace test-id="host" />
        <CSelect v-model:value="transport" :label="t('tools.sip-check.transport')" :options="transports" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.sip-check.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!host.trim()" data-testid="run">
        {{ t('tools.sip-check.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <CCard :title="t('tools.sip-check.probes', { host: data.host })" data-testid="result">
      <div v-for="p in data.probes" :key="p.transport" class="mb-3 border-b nt-border border-b-solid pb-3 last:border-b-0" data-testid="probe">
        <div class="flex flex-wrap items-center gap-2">
          <NTag size="small" :type="stateType[probeState(p)]" :bordered="false">
            {{ p.transport.toUpperCase() }}
          </NTag>
          <span class="font-mono">{{ p.address }}</span>
          <span v-if="p.status" class="font-semibold">{{ p.status }} {{ p.reason }}</span>
          <span v-else>{{ t(`tools.sip-check.errors.${p.error || 'no-reply'}`) }}</span>
          <span v-if="p.rttMs" class="opacity-75">{{ p.rttMs.toFixed(1) }} ms</span>
        </div>
        <div v-if="p.server" class="mt-1 text-sm">
          {{ t('tools.sip-check.server') }}: <span class="font-mono">{{ p.server }}</span>
        </div>
        <div v-if="p.allow.length" class="mt-1 break-all text-sm">
          {{ t('tools.sip-check.allow') }}: <span class="font-mono">{{ p.allow.join(', ') }}</span>
        </div>
        <div v-if="p.cert" class="mt-1 break-all text-sm">
          {{ t('tools.sip-check.cert') }}: <span class="font-mono">{{ p.cert.subject }} · {{ t('tools.sip-check.expires', { days: p.cert.daysLeft }) }}</span>
          <NTag v-if="p.certError" size="tiny" type="warning" :bordered="false" class="ml-1">
            {{ t('tools.sip-check.untrusted') }}
          </NTag>
        </div>
      </div>
      <p class="mb-0 text-sm">
        {{ t('tools.sip-check.statusHint') }}
      </p>
    </CCard>
    <CCard v-if="data.srv.length || data.naptr.length" :title="t('tools.sip-check.discovery')">
      <div v-for="n in data.naptr" :key="n" class="mb-1 break-all text-sm font-mono">
        NAPTR {{ n }}
      </div>
      <ResultTable v-if="srvRows.length" :columns="srvColumns" :rows="srvRows" :actions="false" :label="t('tools.sip-check.discovery')" />
    </CCard>
    <ResultActions :text="() => data!.probes.map(p => `${p.transport}\t${p.address}\t${p.status || p.error}\t${p.server}`).join('\n')" :json="data" filename="sip-check.json" />
  </template>
</template>
