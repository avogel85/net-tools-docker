<script setup lang="ts">
import type { EmailValidate } from './email-validator.service'
import { NAlert, NButton, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import CCard from '@/ui/c-card.vue'
import { counts, MAX_ADDRESSES, splitAddresses } from './email-validator.service'

const { t } = useI18n()
// Addresses are personal data: kept in memory only, not in the URL.
const text = ref('')
const list = computed(() => splitAddresses(text.value))
const tooMany = computed(() => list.value.length > MAX_ADDRESSES)
const { data, error, loading, run } = useApi<EmailValidate>('/v1/email/validate')
const submit = () => list.value.length && !tooMany.value && run({ addresses: list.value.join('\n') })
const summary = computed(() => (data.value ? counts(data.value.results) : undefined))
const verdictType = { valid: 'success', risky: 'warning', invalid: 'error' } as const
const note = (code: string, suggestion: string) => t(`tools.email-validator.findings.${code}`, { suggestion })
const rows = computed(() => (data.value?.results ?? []).map(r => ({
  input: r.input,
  verdict: t(`tools.email-validator.verdicts.${r.verdict}`),
  notes: [...r.problems, ...r.warnings].map(c => note(c, r.suggestion)).join(' · ') || '—',
  mx: r.mx.join(', ') || (r.implicitMx ? t('tools.email-validator.implicit') : '—'),
})))
const columns = computed(() => [
  { key: 'input', title: t('tools.email-validator.address'), mono: true, width: 240 },
  { key: 'verdict', title: t('tools.email-validator.verdict'), width: 110 },
  { key: 'notes', title: t('tools.email-validator.notes'), width: 380 },
  { key: 'mx', title: 'MX', mono: true, width: 260 },
])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <NFormItem :label="t('tools.email-validator.addresses')" :validation-status="tooMany ? 'error' : undefined" :feedback="tooMany ? t('tools.email-validator.tooMany', { max: MAX_ADDRESSES }) : undefined">
        <NInput v-model:value="text" type="textarea" :autosize="{ minRows: 3, maxRows: 14 }" class="font-mono" placeholder="alice@example.com&#10;bob@gmial.com" :input-props="{ 'aria-label': t('tools.email-validator.addresses'), 'spellcheck': false }" data-testid="addresses" />
      </NFormItem>
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.email-validator.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!list.length || tooMany" data-testid="run">
        {{ t('tools.email-validator.run', { n: list.length }) }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data && summary">
    <div class="mb-3 flex flex-wrap gap-2">
      <NTag v-for="v in (['valid', 'risky', 'invalid'] as const)" :key="v" :type="verdictType[v]" :bordered="false">
        {{ t(`tools.email-validator.verdicts.${v}`) }}: {{ summary[v] }}
      </NTag>
    </div>
    <NAlert v-if="data.results.length === 1" :type="verdictType[data.results[0]!.verdict]" class="mb-4" data-testid="single">
      <strong>{{ data.results[0]!.input }}</strong> — {{ t(`tools.email-validator.verdicts.${data.results[0]!.verdict}`) }}
      <ul v-if="data.results[0]!.problems.length + data.results[0]!.warnings.length" class="mb-0 mt-1 pl-5">
        <li v-for="c in [...data.results[0]!.problems, ...data.results[0]!.warnings]" :key="c">
          {{ note(c, data.results[0]!.suggestion) }}
        </li>
      </ul>
    </NAlert>
    <CCard :title="t('common.result')" data-testid="result">
      <ResultTable :columns="columns" :rows="rows" filename="email-validation.json" :label="t('common.result')" />
    </CCard>
  </template>
</template>
