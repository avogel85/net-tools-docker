<script setup lang="ts">
import type { IkeProbe } from './ike-probe.service'
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
import { verdict } from './ike-probe.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const port = useQueryParam({ name: 'port', defaultValue: '500' })
const { data, error, loading, run } = useApi<IkeProbe>('/v1/ike/probe')
const submit = () => host.value.trim() && run({ host: host.value.trim(), port: port.value })
onMounted(() => {
  if (host.value)
    void submit()
})
const v = computed(() => (data.value ? verdict(data.value) : undefined))
const alertType = { 'ikev2': 'success', 'ikev1-only': 'warning', 'answers': 'info', 'silent': 'default' } as const
const ports = [{ value: '500', label: 'UDP 500' }, { value: '4500', label: 'UDP 4500 (NAT-T)' }]
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_14rem]">
        <CInputText v-model:value="host" :label="t('tools.ike-probe.host')" placeholder="vpn.example.com" monospace test-id="host" />
        <CSelect v-model:value="port" :label="t('tools.ike-probe.port')" :options="ports" />
      </div>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.ike-probe.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!host.trim()" data-testid="run">
        {{ t('tools.ike-probe.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data && v">
    <NAlert :type="alertType[v]" class="mb-4" :title="t(`tools.ike-probe.verdict.${v}`)" data-testid="verdict">
      <ul v-if="data.findings.length" class="m-0 pl-5">
        <li v-for="f in data.findings" :key="f">
          {{ t(`tools.ike-probe.findings.${f}`) }}
        </li>
      </ul>
    </NAlert>
    <CCard v-for="ver in (['ikev2', 'ikev1'] as const)" :key="ver" :title="ver === 'ikev2' ? 'IKEv2' : 'IKEv1'" data-testid="result">
      <div class="mb-2 flex flex-wrap gap-2">
        <NTag :type="data[ver].accepted ? 'success' : data[ver].answered ? 'info' : 'default'" :bordered="false">
          {{ data[ver].accepted ? t('tools.ike-probe.accepted') : data[ver].noProposal ? t('tools.ike-probe.noProposal') : data[ver].answered ? t('tools.ike-probe.answered') : t('tools.ike-probe.silent') }}
        </NTag>
        <NTag v-if="ver === 'ikev2' && data.ikev2.fragmentation" size="small" :bordered="false">
          {{ t('tools.ike-probe.fragmentation') }}
        </NTag>
        <NTag v-if="ver === 'ikev2' && data.ikev2.cookie" size="small" :bordered="false">
          {{ t('tools.ike-probe.cookie') }}
        </NTag>
      </div>
      <dl v-if="data[ver].accepted" class="grid grid-cols-[10rem_1fr] m-0 gap-x-4 gap-y-1 text-sm">
        <dt class="font-semibold">
          {{ t('tools.ike-probe.encryption') }}
        </dt>
        <dd class="m-0 font-mono">
          {{ data[ver].encryption }}
        </dd>
        <template v-if="ver === 'ikev2'">
          <dt class="font-semibold">
            PRF
          </dt>
          <dd class="m-0 font-mono">
            {{ data.ikev2.prf }}{{ data.ikev2.integrity ? ` · ${data.ikev2.integrity}` : '' }}
          </dd>
        </template>
        <template v-else>
          <dt class="font-semibold">
            {{ t('tools.ike-probe.hash') }}
          </dt>
          <dd class="m-0 font-mono">
            {{ data.ikev1.hash }}
          </dd>
        </template>
        <dt class="font-semibold">
          {{ t('tools.ike-probe.group') }}
        </dt>
        <dd class="m-0 font-mono">
          {{ data[ver].group }}
          <NTag v-if="data[ver].groupWeak" size="tiny" type="warning" :bordered="false" class="ml-1">
            {{ t('tools.ike-probe.weak') }}
          </NTag>
        </dd>
      </dl>
      <div v-if="data[ver].vendorIds.length" class="mt-2 break-all text-sm">
        {{ t('tools.ike-probe.vendorIds') }}: <span class="font-mono">{{ data[ver].vendorIds.join(' · ') }}</span>
      </div>
    </CCard>
    <ResultActions :text="() => `IKEv2 ${data!.ikev2.accepted ? `${data!.ikev2.encryption} ${data!.ikev2.group}` : '-'}\nIKEv1 ${data!.ikev1.accepted ? `${data!.ikev1.encryption} ${data!.ikev1.group}` : '-'}`" :json="data" filename="ike-probe.json" />
  </template>
</template>
