<script setup lang="ts">
import type { DnsLookalike } from './lookalike-finder.service'
import { NAlert, NButton, NCheckbox } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultTable from '@/components/ResultTable.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { risk, visible } from './lookalike-finder.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })
const { data, error, loading, run } = useApi<DnsLookalike>('/v1/dns/lookalike')
const submit = () => domain.value && validation.value.isValid && run({ domain: domain.value })
onMounted(() => {
  if (domain.value)
    void submit()
})
const registeredOnly = ref(true)
const kind = ref('')
const kinds = ['tld', 'homoglyph', 'idn', 'transposition', 'omission', 'repetition', 'hyphenation', 'prefix', 'suffix', 'replacement', 'bitsquatting']
const kindOptions = computed(() => [{ value: '', label: t('tools.lookalike-finder.allKinds') }, ...kinds.map(k => ({ value: k, label: t(`tools.lookalike-finder.kinds.${k}`) }))])
const registered = computed(() => data.value?.candidates.filter(c => c.registered) ?? [])
const rows = computed(() => (data.value ? visible(data.value.candidates, registeredOnly.value, kind.value) : []).map(c => ({
  domain: c.unicode ? `${c.unicode} (${c.domain})` : c.domain,
  kind: t(`tools.lookalike-finder.kinds.${c.kind}`),
  status: c.error ? t('tools.lookalike-finder.error') : c.registered ? t(`tools.lookalike-finder.risk.${risk(c)}`) : t('tools.lookalike-finder.free'),
  a: c.a.join(', ') || '—',
  mx: c.mx.join(', ') || '—',
})))
const columns = computed(() => [
  { key: 'domain', title: t('tools.lookalike-finder.domain'), mono: true, width: 260 },
  { key: 'kind', title: t('tools.lookalike-finder.kind'), width: 140 },
  { key: 'status', title: t('tools.lookalike-finder.status'), width: 170 },
  { key: 'a', title: 'A / AAAA', mono: true, width: 200 },
  { key: 'mx', title: 'MX', mono: true, width: 220 },
])
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="domain" :label="t('tools.lookalike-finder.yourDomain')" placeholder="example.com" :validation="validation" monospace test-id="domain" />
      <p class="mb-3 mt-0 text-sm">
        {{ t('tools.lookalike-finder.hint') }}
      </p>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!domain || !validation.isValid" data-testid="run">
        {{ t('tools.lookalike-finder.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <NAlert :type="registered.some(c => c.mx.length) ? 'warning' : 'info'" class="mb-4" data-testid="summary">
      {{ t('tools.lookalike-finder.summary', { checked: data.candidates.length, registered: registered.length, mx: registered.filter(c => c.mx.length).length }) }}
      <template v-if="data.truncated">
        {{ t('tools.lookalike-finder.truncated', { n: data.generated }) }}
      </template>
    </NAlert>
    <CCard :title="t('common.result')" data-testid="result">
      <div class="mb-3 flex flex-wrap items-center gap-x-6 gap-y-2">
        <NCheckbox v-model:checked="registeredOnly">
          {{ t('tools.lookalike-finder.registeredOnly') }}
        </NCheckbox>
        <div class="min-w-48">
          <CSelect v-model:value="kind" :options="kindOptions" :label="t('tools.lookalike-finder.kind')" />
        </div>
      </div>
      <ResultTable :columns="columns" :rows="rows" filename="lookalikes.json" :label="t('common.result')" />
    </CCard>
  </template>
</template>
