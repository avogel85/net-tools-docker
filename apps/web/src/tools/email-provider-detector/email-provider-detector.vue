<script setup lang="ts">
import type { EmailProviders } from './email-provider-detector.service'
import { NAlert, NButton, NTag } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ApiError from '@/components/ApiError.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useApi } from '@/composables/useApi'
import { useQueryParam } from '@/composables/useQueryParam'
import { useValidation } from '@/composables/useValidation'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { isValidQueryName } from '../dns-lookup/dns-lookup.service'
import { groupDetections } from './email-provider-detector.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const validation = useValidation({ source: domain, rules: [{ validator: isValidQueryName, message: t('tools.dns-lookup.invalidName') }] })
const { data, error, loading, run } = useApi<EmailProviders>('/v1/email/providers')
const submit = () => domain.value && validation.value.isValid && run({ domain: domain.value })
onMounted(() => {
  if (domain.value)
    void submit()
})
const groups = computed(() => (data.value ? groupDetections(data.value.detections) : []))
const report = () => groups.value.map(g => `${t(`tools.email-provider-detector.roles.${g.role}`)}: ${g.providers.map(p => p.provider).join(', ')}`).join('\n')
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="submit">
      <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" :validation="validation" monospace test-id="domain" />
      <NButton type="primary" attr-type="submit" :loading="loading" :disabled="!domain || !validation.isValid" data-testid="run">
        {{ t('tools.email-provider-detector.run') }}
      </NButton>
    </form>
  </CCard>
  <ApiError v-if="error" :error="error" />
  <template v-if="data">
    <NAlert v-if="!groups.length" type="info" class="mb-4" data-testid="none">
      {{ t('tools.email-provider-detector.none') }}
    </NAlert>
    <CCard v-for="g in groups" :key="g.role" :title="t(`tools.email-provider-detector.roles.${g.role}`)" data-testid="result">
      <p class="mt-0 text-sm">
        {{ t(`tools.email-provider-detector.roleHints.${g.role}`) }}
      </p>
      <div v-for="p in g.providers" :key="p.provider" class="mb-2 flex flex-wrap items-start gap-2">
        <NTag :bordered="false" type="info" class="shrink-0">
          {{ p.provider }}
        </NTag>
        <span class="break-all text-sm font-mono">{{ p.evidence.join(' · ') }}</span>
      </div>
    </CCard>
    <CCard :title="t('tools.email-provider-detector.records')">
      <dl class="grid grid-cols-[auto_1fr] m-0 gap-x-4 gap-y-1 text-sm">
        <dt class="font-semibold">
          MX
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ data.mx.join(', ') || '—' }}
        </dd>
        <dt class="font-semibold">
          SPF
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ data.spf || '—' }}
        </dd>
        <dt class="font-semibold">
          DMARC
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ data.dmarc || '—' }}
        </dd>
        <dt class="font-semibold">
          DKIM
        </dt>
        <dd class="m-0 break-all font-mono">
          {{ data.selectors.join(', ') || '—' }}
        </dd>
      </dl>
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.email-provider-detector.limits') }}
      </p>
      <div class="mt-3">
        <ResultActions :text="report" :json="data" filename="email-providers.json" />
      </div>
    </CCard>
  </template>
</template>
