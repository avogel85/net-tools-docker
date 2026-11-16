<script setup lang="ts">
import type { EmailSpfFlatten } from './spf-flattener.service'
import { NAlert, NButton, NFormItem, NInput, NInputNumber, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import CopyButton from '@/components/CopyButton.vue'
import FindingsList from '@/components/FindingsList.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { DEFAULT_LABEL, DEFAULT_SIZE, isValidLabel, zoneText } from './spf-flattener.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const label = useQueryParam({ name: 'label', defaultValue: DEFAULT_LABEL })
const size = useQueryParam({ name: 'size', defaultValue: DEFAULT_SIZE })
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })
const labelValidation = useValidation({ source: label, rules: [{ validator: isValidLabel, message: t('tools.spf-flattener.invalidLabel') }] })
const sizeValid = computed(() => Number.isInteger(size.value) && size.value >= 100 && size.value <= 2048)
const canRun = computed(() => !!domain.value && validation.value.isValid && labelValidation.value.isValid && sizeValid.value)

const { data, error, loading, run } = useApi<EmailSpfFlatten>('/v1/email/spf-flatten')
function submit() {
  if (canRun.value)
    void run({ domain: domain.value, label: label.value.toLowerCase(), size: String(size.value) })
}
onMounted(() => {
  if (domain.value)
    submit()
})

const zone = computed(() => (data.value ? zoneText(data.value.records) : ''))
const sizeModel = computed({
  get: () => size.value,
  set: (v: number | null) => { size.value = v ?? DEFAULT_SIZE },
})
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" :validation="validation" monospace test-id="domain" />
      <div class="flex flex-wrap gap-x-4">
        <NFormItem :label="t('tools.spf-flattener.label')" class="max-w-48" :validation-status="labelValidation.isValid ? undefined : 'error'" :feedback="labelValidation.isValid ? undefined : t('tools.spf-flattener.invalidLabel')">
          <NInput v-model:value="label" :placeholder="DEFAULT_LABEL" class="font-mono" :input-props="{ 'aria-label': t('tools.spf-flattener.label'), 'data-testid': 'label' } as Record<string, string>" />
        </NFormItem>
        <NFormItem :label="t('tools.spf-flattener.size')" class="max-w-48" :validation-status="sizeValid ? undefined : 'error'" :feedback="sizeValid ? undefined : t('tools.spf-flattener.invalidSize')">
          <NInputNumber v-model:value="sizeModel" :min="100" :max="2048" :step="50" :show-button="false" :input-props="{ 'aria-label': t('tools.spf-flattener.size') }" data-testid="size" />
        </NFormItem>
      </div>
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!canRun" data-testid="run">
        {{ t('tools.spf-flattener.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <FindingsList :problems="data.problems" :warnings="data.warnings" :ok-text="t('tools.spf-flattener.ok')" />
    <CCard v-if="data.records.length" :title="t('tools.spf-flattener.records')" data-testid="records">
      <p class="mt-0 text-sm">
        {{ t('tools.spf-flattener.summary', { before: data.originalLookups, after: data.lookups, ip4: data.ip4, ip6: data.ip6, records: data.records.length }) }}
      </p>
      <div v-for="r in data.records" :key="r.name" class="mb-3" data-testid="record">
        <div class="mb-1 flex flex-wrap items-center gap-2">
          <span class="break-all text-sm font-semibold font-mono">{{ r.name }}</span>
          <NTag size="tiny" :bordered="false" :type="r.length > size ? 'error' : 'default'">
            {{ t('tools.spf-flattener.bytes', { n: r.length }) }}
          </NTag>
          <CopyButton :text="r.value" size="tiny" />
        </div>
        <p class="m-0 break-all text-sm font-mono">
          {{ r.value }}
        </p>
      </div>
      <h3 class="mb-2 text-sm font-semibold">
        {{ t('tools.spf-flattener.zone') }}
      </h3>
      <pre class="m-0 mb-3 overflow-x-auto whitespace-pre-wrap break-all text-xs font-mono" data-testid="zone">{{ zone }}</pre>
      <NAlert type="info" :title="t('tools.spf-flattener.staleTitle')" class="mb-3">
        {{ t('tools.spf-flattener.stale') }}
      </NAlert>
      <ResultActions :text="() => zone" :json="data" filename="spf-flattened.json" />
    </CCard>
    <CCard v-if="data.sources.length" :title="t('tools.spf-flattener.sources')" data-testid="sources">
      <p class="mt-0 break-all text-sm font-mono">
        {{ data.original }}
      </p>
      <ul class="m-0 list-none p-0 text-sm">
        <li v-for="(s, i) in data.sources" :key="i" class="py-1">
          <span class="font-mono">{{ s.term }}</span>
          <NTag v-if="s.kept" size="tiny" :bordered="false" type="warning" class="ml-1">
            {{ t('tools.spf-flattener.kept') }}
          </NTag>
          <span v-else-if="s.prefixes.length" class="ml-1 text-xs opacity-75">→ {{ s.prefixes.join(' ') }}</span>
        </li>
      </ul>
    </CCard>
  </template>
</template>
