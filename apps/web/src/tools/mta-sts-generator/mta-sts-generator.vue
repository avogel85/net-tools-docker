<script setup lang="ts">
import type { Mode } from './mta-sts-generator.service'
import { NAlert, NButton, NFormItem, NInput, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import { recordZoneText } from '@/components/record-list'
import RecordList from '@/components/RecordList.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { downloadText } from '@/utils/download'
import { splitList } from '@/utils/ip'
import { buildMtaSts, policyId } from './mta-sts-generator.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const mode = useQueryParam({ name: 'mode', defaultValue: 'testing' })
const mx = useQueryParam({ name: 'mx', defaultValue: '' })
const maxAge = useQueryParam({ name: 'max_age', defaultValue: 604800 })
const rua = useQueryParam({ name: 'rua', defaultValue: '' })
const id = useQueryParam({ name: 'id', defaultValue: policyId() })

const modes: Mode[] = ['testing', 'enforce', 'none']
const ages = computed(() => [86400, 604800, 2592000, 31557600].map(v => ({ value: v, label: t(`tools.mta-sts-generator.ages.${v}`) })))
const result = computed(() => buildMtaSts({ domain: domain.value, mode: mode.value as Mode, mx: splitList(mx.value), maxAge: maxAge.value, rua: splitList(rua.value), id: id.value }))
const err = (code: string, msg: string) => (result.value.errors.includes(code) ? { isValid: false, status: 'error' as const, message: msg } : undefined)
const allText = computed(() => `${recordZoneText(result.value.records)}\n\n# ${result.value.policyUrl}\n${result.value.policy.replace(/\r\n/g, '\n')}`)
</script>

<template>
  <CCard>
    <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" monospace test-id="domain" :validation="domain ? err('domain', t('tools.dns-lookup.invalidName')) : undefined" />
    <NFormItem :label="t('tools.mta-sts-generator.mode')">
      <NRadioGroup v-model:value="mode" name="mode">
        <NRadioButton v-for="m in modes" :key="m" :value="m" :data-testid="`mode-${m}`">
          {{ t(`tools.mta-sts-generator.modes.${m}`) }}
        </NRadioButton>
      </NRadioGroup>
    </NFormItem>
    <CInputText v-model:value="mx" :label="t('tools.mta-sts-generator.mx')" placeholder="mx1.example.com, *.mail.protection.outlook.com" monospace test-id="mx" :validation="err('mx', t('tools.mta-sts-generator.invalidMx')) ?? (domain ? err('mx-missing', t('tools.mta-sts-generator.mxMissing')) : undefined)" />
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CSelect v-model:value="maxAge" :label="t('tools.mta-sts-generator.maxAge')" :options="ages" />
      <CInputText v-model:value="id" :label="t('tools.mta-sts-generator.id')" monospace test-id="id" :validation="err('id', t('tools.mta-sts-generator.invalidId'))" />
    </div>
    <NButton secondary size="small" class="mb-4" @click="id = policyId()">
      {{ t('tools.mta-sts-generator.newId') }}
    </NButton>
    <CInputText v-model:value="rua" :label="t('tools.mta-sts-generator.rua')" placeholder="tls-reports@example.com" monospace test-id="rua" :validation="err('rua', t('tools.mta-sts-generator.invalidRua'))" />
  </CCard>

  <template v-if="domain && !result.errors.length">
    <div class="mb-4 flex flex-col gap-2">
      <NAlert v-for="w in result.warnings" :key="w" type="warning">
        {{ t(`tools.mta-sts-generator.findings.${w}`) }}
      </NAlert>
    </div>
    <CCard :title="t('tools.mta-sts-generator.records')" data-testid="result">
      <RecordList :records="result.records" />
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.mta-sts-generator.hostHint', { host: `mta-sts.${domain}` }) }}
      </p>
    </CCard>
    <CCard :title="t('tools.mta-sts-generator.policy')" data-testid="policy">
      <p class="mt-0 break-all text-sm font-mono">
        {{ result.policyUrl }}
      </p>
      <NInput :value="result.policy.replace(/\r\n/g, '\n')" type="textarea" readonly :autosize="{ minRows: 4 }" class="font-mono" :input-props="{ 'aria-label': t('tools.mta-sts-generator.policy') }" />
      <div class="mt-2 flex flex-wrap gap-2">
        <CopyButton :text="result.policy" />
        <NButton secondary size="small" @click="downloadText('mta-sts.txt', result.policy, 'text/plain')">
          {{ t('tools.mta-sts-generator.download') }}
        </NButton>
      </div>
    </CCard>
    <CCard :title="t('tools.mta-sts-generator.nginx')">
      <pre class="m-0 mb-2 overflow-x-auto text-xs font-mono">{{ result.nginx }}</pre>
      <CopyButton :text="result.nginx" />
    </CCard>
    <ResultActions :text="() => allText" :json="result" filename="mta-sts.json" />
  </template>
</template>
