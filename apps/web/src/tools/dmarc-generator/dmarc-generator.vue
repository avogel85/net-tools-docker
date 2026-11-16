<script setup lang="ts">
import type { Alignment, FailureOption, Policy } from './dmarc-generator.service'
import { NAlert, NButton, NCheckbox, NCheckboxGroup, NFormItem, NInput, NInputNumber, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { recordZoneText } from '@/components/record-list'
import RecordList from '@/components/RecordList.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { splitList } from '@/utils/ip'
import { buildDmarc, DEFAULTS, parseDmarc } from './dmarc-generator.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const p = useQueryParam({ name: 'p', defaultValue: DEFAULTS.p as string })
const sp = useQueryParam({ name: 'sp', defaultValue: '' })
const adkim = useQueryParam({ name: 'adkim', defaultValue: 'r' })
const aspf = useQueryParam({ name: 'aspf', defaultValue: 'r' })
const pct = useQueryParam({ name: 'pct', defaultValue: 100 })
const rua = useQueryParam({ name: 'rua', defaultValue: '' })
const ruf = useQueryParam({ name: 'ruf', defaultValue: '' })
const fo = useQueryParam({ name: 'fo', defaultValue: '0' })
const ri = useQueryParam({ name: 'ri', defaultValue: 86400 })

const policies: Policy[] = ['none', 'quarantine', 'reject']
const foModel = computed({
  get: () => fo.value.split(':').filter(Boolean),
  set: (v: (string | number)[]) => { fo.value = v.length ? v.map(String).join(':') : '0' },
})
const pctModel = computed({
  get: () => pct.value,
  set: (v: number | null) => {
    pct.value = v ?? 100
  },
})

const result = computed(() => buildDmarc({
  domain: domain.value,
  p: p.value as Policy,
  sp: sp.value as Policy | '',
  adkim: adkim.value as Alignment,
  aspf: aspf.value as Alignment,
  pct: pct.value,
  rua: splitList(rua.value),
  ruf: splitList(ruf.value),
  fo: foModel.value as FailureOption[],
  ri: ri.value,
}))
const records = computed(() => [result.value.record, ...result.value.authorizations])

const imported = ref('')
const importError = ref(false)
function load() {
  const input = parseDmarc(imported.value, domain.value)
  importError.value = !input
  if (!input)
    return
  p.value = input.p
  sp.value = input.sp
  adkim.value = input.adkim
  aspf.value = input.aspf
  pct.value = input.pct
  rua.value = input.rua.join(', ')
  ruf.value = input.ruf.join(', ')
  fo.value = input.fo.join(':')
  ri.value = input.ri
}
const intervals = computed(() => [3600, 4 * 3600, 86400].map(s => ({ value: s, label: t(`tools.dmarc-generator.interval.${s}`) })))
const warningText = (code: string) => (['ruf-privacy', 'sp-none'].includes(code) ? t(`tools.dmarc-generator.findings.${code}`) : t(`email.findings.${code}`))
</script>

<template>
  <CCard>
    <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" monospace test-id="domain" :validation="result.errors.includes('domain') && domain ? { isValid: false, status: 'error', message: t('tools.dns-lookup.invalidName') } : undefined" />
    <NFormItem :label="t('tools.dmarc-generator.policy')">
      <NRadioGroup v-model:value="p" name="p">
        <NRadioButton v-for="v in policies" :key="v" :value="v" :data-testid="`p-${v}`">
          {{ t(`tools.dmarc-generator.policies.${v}`) }}
        </NRadioButton>
      </NRadioGroup>
    </NFormItem>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-3">
      <CSelect v-model:value="sp" :label="t('tools.dmarc-generator.sp')" :options="[{ value: '', label: t('tools.dmarc-generator.inherit') }, ...policies.map(v => ({ value: v, label: t(`tools.dmarc-generator.policies.${v}`) }))]" />
      <CSelect v-model:value="adkim" :label="t('tools.dmarc-generator.adkim')" :options="[{ value: 'r', label: t('tools.dmarc-generator.relaxed') }, { value: 's', label: t('tools.dmarc-generator.strict') }]" />
      <CSelect v-model:value="aspf" :label="t('tools.dmarc-generator.aspf')" :options="[{ value: 'r', label: t('tools.dmarc-generator.relaxed') }, { value: 's', label: t('tools.dmarc-generator.strict') }]" />
    </div>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <NFormItem :label="t('tools.dmarc-generator.pct')" :validation-status="result.errors.includes('pct') ? 'error' : undefined">
        <NInputNumber v-model:value="pctModel" :min="0" :max="100" :show-button="false" :input-props="{ 'aria-label': t('tools.dmarc-generator.pct') }">
          <template #suffix>
            %
          </template>
        </NInputNumber>
      </NFormItem>
      <CSelect v-model:value="ri" :label="t('tools.dmarc-generator.ri')" :options="intervals" />
    </div>
    <CInputText v-model:value="rua" :label="t('tools.dmarc-generator.rua')" placeholder="dmarc@example.com" monospace test-id="rua" :validation="result.errors.includes('rua') ? { isValid: false, status: 'error', message: t('tools.dmarc-generator.invalidAddress') } : undefined" />
    <CInputText v-model:value="ruf" :label="t('tools.dmarc-generator.ruf')" placeholder="forensic@example.com" monospace test-id="ruf" :validation="result.errors.includes('ruf') ? { isValid: false, status: 'error', message: t('tools.dmarc-generator.invalidAddress') } : undefined" />
    <NFormItem v-if="ruf" :label="t('tools.dmarc-generator.fo')">
      <NCheckboxGroup v-model:value="foModel">
        <div class="flex flex-wrap gap-x-4 gap-y-1">
          <NCheckbox v-for="o in ['0', '1', 'd', 's']" :key="o" :value="o" :label="t(`tools.dmarc-generator.foOptions.${o}`)" />
        </div>
      </NCheckboxGroup>
    </NFormItem>
  </CCard>

  <CCard :title="t('common.result')" data-testid="result">
    <div class="mb-3 flex flex-col gap-2">
      <NAlert v-for="w in result.warnings" :key="w" type="warning">
        {{ warningText(w) }}
      </NAlert>
    </div>
    <RecordList :records="records" />
    <p v-if="result.authorizations.length" class="mb-0 mt-3 text-sm">
      {{ t('tools.dmarc-generator.authorizationHint') }}
    </p>
    <div class="mt-3">
      <ResultActions :text="() => recordZoneText(records)" :json="result" filename="dmarc.json" />
    </div>
  </CCard>

  <CCard :title="t('tools.dmarc-generator.import')">
    <NInput v-model:value="imported" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" class="font-mono" placeholder="v=DMARC1; p=quarantine; rua=mailto:…" :input-props="{ 'aria-label': t('tools.dmarc-generator.import'), 'spellcheck': false }" data-testid="import" />
    <div class="mt-2 flex items-center gap-3">
      <NButton secondary :disabled="!imported.trim()" data-testid="load" @click="load">
        {{ t('tools.dmarc-generator.load') }}
      </NButton>
      <span v-if="importError" class="text-sm" role="alert">{{ t('tools.dmarc-generator.importError') }}</span>
    </div>
  </CCard>
</template>
