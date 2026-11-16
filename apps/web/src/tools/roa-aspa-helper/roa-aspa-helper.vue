<script setup lang="ts">
import type { RoaCoverage } from '../roa-coverage/roa-coverage.service'
import { NAlert, NButton, NFormItem, NInput, NRadioButton, NRadioGroup, NTag } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import CopyButton from '@/components/CopyButton.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { parseAsn } from '@/utils/bgp'
import { krillAspa, krillDelta, parseAspas, parsePath, roaCsv, roaPlan, verify } from './roa-aspa-helper.service'

const { t } = useI18n()

// ROA plan
const asn = useQueryParam({ name: 'asn', defaultValue: '' })
const input = ref(asn.value)
const validation = useValidation({ source: input, rules: [{ validator: (v: string) => parseAsn(v) !== undefined, message: t('bgp.invalidAsn') }] })
const { data, error, loading, run } = useApi<RoaCoverage>('/v1/bgp/roa-coverage')
function submit() {
  const n = parseAsn(input.value)
  if (n === undefined)
    return
  if (asn.value === `AS${n}`)
    void run({}, `/AS${n}`)
  asn.value = `AS${n}`
}
watch(asn, (v) => {
  input.value = v
  const n = parseAsn(v)
  if (n !== undefined)
    void run({}, `/AS${n}`)
}, { immediate: true })
const plan = computed(() => (data.value ? roaPlan(data.value) : []))
const format = ref<'krill' | 'csv'>('krill')
const planText = computed(() => (data.value ? (format.value === 'krill' ? krillDelta(data.value.asn, plan.value) : roaCsv(data.value.asn, plan.value)) : ''))
const planRows = computed(() => plan.value.map(c => ({
  action: t(`tools.roa-aspa-helper.actions.${c.action}`),
  roa: `${c.prefix} max /${c.maxLength}`,
  why: t(`tools.roa-aspa-helper.why.${c.why}`),
})))

// ASPA
const records = useQueryParam({ name: 'aspa', defaultValue: '' })
const path = useQueryParam({ name: 'path', defaultValue: '' })
const direction = useQueryParam({ name: 'dir', defaultValue: 'upstream' })
const parsed = computed(() => parseAspas(records.value))
const asPath = computed(() => parsePath(path.value))
const result = computed(() => (asPath.value ? verify(parsed.value.aspas, asPath.value, direction.value === 'downstream' ? 'downstream' : 'upstream') : undefined))
const commands = computed(() => [...parsed.value.aspas].map(([c, ps]) => krillAspa(c, [...ps].filter(p => p !== 0))).join(''))
const verdictType = { valid: 'success', invalid: 'error', unknown: 'warning' } as const
</script>

<template>
  <CCard :title="t('tools.roa-aspa-helper.roaTitle')">
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="input" :label="t('bgp.asn')" placeholder="AS3333" :validation="validation" monospace test-id="asn" />
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!validation.isValid" data-testid="run">
        {{ t('tools.roa-aspa-helper.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <CCard v-if="data" :title="t('tools.roa-aspa-helper.plan', { asn: `AS${data.asn}`, name: data.name })" data-testid="result">
    <p v-if="!plan.length" class="m-0" data-testid="plan-empty">
      {{ t('tools.roa-aspa-helper.nothing') }}
    </p>
    <template v-else>
      <ResultTable :columns="[{ key: 'action', title: t('tools.roa-aspa-helper.cols.action'), width: 110 }, { key: 'roa', title: 'ROA', mono: true, width: 240 }, { key: 'why', title: t('tools.roa-aspa-helper.cols.why'), width: 360 }]" :rows="planRows" filename="roa-plan.json" />
      <NRadioGroup v-model:value="format" name="format" size="small" class="my-3">
        <NRadioButton value="krill">
          Krill
        </NRadioButton>
        <NRadioButton value="csv">
          CSV
        </NRadioButton>
      </NRadioGroup>
      <div class="flex items-start gap-2">
        <pre class="m-0 min-w-0 flex-1 overflow-auto text-xs font-mono" tabindex="0" data-testid="plan-text">{{ planText }}</pre>
        <CopyButton :text="planText" :label="t('common.copyText')" />
      </div>
    </template>
    <p class="mb-0 mt-3 text-sm">
      {{ t('tools.roa-aspa-helper.roaHint') }}
    </p>
  </CCard>

  <CCard :title="t('tools.roa-aspa-helper.aspaTitle')">
    <NFormItem :label="t('tools.roa-aspa-helper.records')" :validation-status="parsed.invalid.length ? 'error' : undefined" :feedback="parsed.invalid.length ? t('tools.roa-aspa-helper.invalidRecords', { list: parsed.invalid.join('; ') }) : undefined">
      <NInput v-model:value="records" type="textarea" :autosize="{ minRows: 3, maxRows: 10 }" class="font-mono" placeholder="AS64500: AS64510, AS64511&#10;AS64510: AS64520" :input-props="{ 'aria-label': t('tools.roa-aspa-helper.records'), 'spellcheck': false }" data-testid="records" />
    </NFormItem>
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_auto]">
      <CInputText v-model:value="path" :label="t('tools.roa-aspa-helper.path')" placeholder="64520 64510 64500" monospace test-id="path" :validation="path.trim() && !asPath ? { isValid: false, status: 'error', message: t('tools.roa-aspa-helper.invalidPath') } : undefined" />
      <NFormItem :label="t('tools.roa-aspa-helper.direction')">
        <NRadioGroup v-model:value="direction" name="dir">
          <NRadioButton value="upstream">
            {{ t('tools.roa-aspa-helper.dirs.upstream') }}
          </NRadioButton>
          <NRadioButton value="downstream">
            {{ t('tools.roa-aspa-helper.dirs.downstream') }}
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
    </div>
    <div v-if="result" class="mt-1">
      <NTag :type="verdictType[result.verdict]" :bordered="false" data-testid="verdict">
        {{ t(`tools.roa-aspa-helper.verdicts.${result.verdict}`) }}
      </NTag>
      <ul class="m-0 mt-2 pl-5 text-sm font-mono">
        <li v-for="h in result.hops" :key="`${h.from}-${h.to}`">
          AS{{ h.from }} → AS{{ h.to }}: {{ t(`tools.roa-aspa-helper.hops.${h.hop}`) }}
        </li>
      </ul>
      <p class="mb-0 mt-2 text-sm">
        {{ t(`tools.roa-aspa-helper.explain.${result.verdict}`) }}
      </p>
    </div>
    <template v-if="commands">
      <div class="mb-1 mt-4 text-sm font-semibold">
        {{ t('tools.roa-aspa-helper.publish') }}
      </div>
      <div class="flex items-start gap-2">
        <pre class="m-0 min-w-0 flex-1 overflow-auto text-xs font-mono" tabindex="0">{{ commands }}</pre>
        <CopyButton :text="commands" :label="t('common.copyText')" />
      </div>
    </template>
    <NAlert type="info" class="mt-4">
      {{ t('tools.roa-aspa-helper.aspaHint') }}
    </NAlert>
  </CCard>
</template>
