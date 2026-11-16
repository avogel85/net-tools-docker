<script setup lang="ts">
import type { RemoteAccess } from './remote-access-check.service'
import { NAlert, NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { state } from './remote-access-check.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const service = useQueryParam({ name: 'service', defaultValue: 'all' })
const { data, error, loading, run } = useApi<RemoteAccess>('/v1/remote-access/check')
const submit = () => host.value.trim() && run({ host: host.value.trim(), service: service.value })
onMounted(() => {
  if (host.value)
    void submit()
})
const services = computed(() => ['all', 'rdp', 'vnc', 'ssh'].map(v => ({ value: v, label: t(`tools.remote-access-check.services.${v}`) })))
const stateType = { exposed: 'error', open: 'warning', closed: 'success' } as const
const anyOpen = computed(() => data.value?.services.some(s => s.open))
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_14rem]">
        <CInputText v-model:value="host" :label="t('tools.remote-access-check.host')" placeholder="vpn.example.com, 203.0.113.10:3390" monospace test-id="host" />
        <CSelect v-model:value="service" :label="t('tools.remote-access-check.service')" :options="services" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.remote-access-check.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!host.trim()" data-testid="run">
        {{ t('tools.remote-access-check.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <NAlert v-if="anyOpen" type="warning" class="mb-4" data-testid="exposure">
      {{ t('tools.remote-access-check.exposed') }}
    </NAlert>
    <CCard :title="data.host" data-testid="result">
      <div v-for="s in data.services" :key="s.service" class="mb-3 border-b nt-border border-b-solid pb-3 last:border-b-0">
        <div class="flex flex-wrap items-center gap-2">
          <NTag size="small" :type="stateType[state(s)]" :bordered="false">
            {{ s.service.toUpperCase() }}
          </NTag>
          <span class="font-mono">{{ s.address }}</span>
          <span>{{ s.open ? (s.error ? t(`tools.remote-access-check.errors.${s.error}`) : t('tools.remote-access-check.open')) : t(`tools.remote-access-check.errors.${s.error || 'refused'}`) }}</span>
        </div>
        <div v-if="s.protocol" class="mt-1 text-sm">
          {{ t('tools.remote-access-check.protocol') }}: <span class="font-mono">{{ t(`tools.remote-access-check.protocols.${s.protocol}`) }}</span>
          <template v-if="s.nlaRequired !== undefined">
            · NLA {{ s.nlaRequired ? t('tools.remote-access-check.required') : t('tools.remote-access-check.optional') }}
          </template>
        </div>
        <div v-if="s.cert" class="mt-1 break-all text-sm">
          {{ t('tools.remote-access-check.cert') }}: <span class="font-mono">{{ s.cert.subject }}</span>{{ s.certError ? ` (${t('tools.remote-access-check.untrusted')})` : '' }}
        </div>
        <div v-if="s.rfbVersion" class="mt-1 text-sm">
          RFB {{ s.rfbVersion }} · <span class="font-mono">{{ s.securityTypes.join(', ') || '—' }}</span>
        </div>
        <div v-if="s.banner" class="mt-1 break-all text-sm font-mono">
          {{ s.banner }}
        </div>
        <NAlert v-for="f in s.findings" :key="f" :type="f === 'vnc-no-auth' || f === 'ssh-v1' ? 'error' : 'warning'" class="mt-2">
          {{ t(`tools.remote-access-check.findings.${f}`) }}
        </NAlert>
      </div>
      <ResultActions :text="() => data!.services.map(s => `${s.service}\t${s.address}\t${s.open ? 'open' : 'closed'}\t${s.findings.join(',')}`).join('\n')" :json="data" filename="remote-access.json" />
    </CCard>
  </template>
</template>
