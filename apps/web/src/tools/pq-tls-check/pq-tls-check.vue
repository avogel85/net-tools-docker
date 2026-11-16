<script setup lang="ts">
import type { TlsPq } from './pq-tls-check.service'
import { NAlert, NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { verdict } from './pq-tls-check.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const port = useQueryParam({ name: 'port', defaultValue: '443' })
const portOk = computed(() => /^\d{1,5}$/.test(port.value) && Number(port.value) >= 1 && Number(port.value) <= 65535)
const { data, error, loading, run } = useApi<TlsPq>('/v1/tls/pq')
const submit = () => host.value.trim() && portOk.value && run({ host: host.value.trim(), port: port.value })
onMounted(() => {
  if (host.value)
    void submit()
})
const v = computed(() => (data.value ? verdict(data.value) : undefined))
const alertType = { 'pq': 'success', 'pq-available': 'info', 'classic': 'warning', 'unreachable': 'error' } as const
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_8rem]">
        <CInputText v-model:value="host" :label="t('tools.pq-tls-check.host')" placeholder="www.example.com" monospace test-id="host" />
        <CInputText v-model:value="port" :label="t('tools.pq-tls-check.port')" placeholder="443" monospace test-id="port" :validation="portOk ? undefined : { isValid: false, status: 'error', message: '1–65535' }" />
      </div>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!host.trim() || !portOk" data-testid="run">
        {{ t('tools.pq-tls-check.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data && v">
    <NAlert :type="alertType[v]" class="mb-4" :title="t(`tools.pq-tls-check.verdict.${v}`)" data-testid="verdict">
      {{ v === 'unreachable' ? t(`tools.pq-tls-check.errors.${data.error}`) : t(`tools.pq-tls-check.explain.${v}`, { group: data.negotiated, version: data.version }) }}
    </NAlert>
    <CCard v-if="data.groups.length" :title="t('tools.pq-tls-check.groups')" data-testid="result">
      <ul class="m-0 list-none p-0 text-sm">
        <li v-for="g in data.groups" :key="g.name" class="flex flex-wrap items-center gap-2 py-1">
          <NTag size="small" :type="g.supported ? (g.pq ? 'success' : 'default') : 'default'" :bordered="false" class="min-w-24 justify-center">
            {{ g.supported ? t('tools.pq-tls-check.supported') : t('tools.pq-tls-check.notSupported') }}
          </NTag>
          <span class="font-mono">{{ g.name }}</span>
          <span class="opacity-75">{{ g.pq ? t('tools.pq-tls-check.hybrid') : t('tools.pq-tls-check.classical') }}</span>
        </li>
      </ul>
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.pq-tls-check.hint', { address: data.address }) }}
      </p>
      <div class="mt-3">
        <ResultActions :text="() => data!.groups.map(g => `${g.name}\t${g.supported ? 'yes' : 'no'}`).join('\n')" :json="data" filename="pq-tls.json" />
      </div>
    </CCard>
  </template>
</template>
