<script setup lang="ts">
import type { DkimKeys, KeyKind } from './dkim-generator.service'
import { NAlert, NButton, NCheckbox, NFormItem, NInput, NRadioButton, NRadioGroup } from 'naive-ui'
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import RecordList from '@/components/RecordList.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { downloadText } from '@/utils/download'
import { buildDkimRecord, generateKeys, isValidDomain, isValidSelector } from './dkim-generator.service'

const { t } = useI18n()
const domain = useQueryParam({ name: 'domain', defaultValue: '' })
const selector = useQueryParam({ name: 'selector', defaultValue: `s${new Date().toISOString().slice(0, 7).replace('-', '')}` })
const kind = useQueryParam({ name: 'type', defaultValue: 'rsa-2048' })
const testing = useQueryParam({ name: 'testing', defaultValue: false })
const strict = useQueryParam({ name: 'strict', defaultValue: false })

const domainOk = computed(() => isValidDomain(domain.value))
const selectorOk = computed(() => isValidSelector(selector.value))
const kinds: KeyKind[] = ['rsa-2048', 'rsa-1024', 'rsa-4096', 'ed25519']

// Keys stay in memory only: never in the URL or browser storage.
const keys = shallowRef<DkimKeys>()
const busy = ref(false)
const failed = ref(false)
watch(kind, () => {
  keys.value = undefined
})
async function generate() {
  busy.value = true
  failed.value = false
  try {
    keys.value = await generateKeys(kind.value as KeyKind)
  }
  catch {
    keys.value = undefined
    failed.value = true
  }
  finally {
    busy.value = false
  }
}
const record = computed(() => keys.value && domainOk.value && selectorOk.value
  ? buildDkimRecord({ selector: selector.value, domain: domain.value, kind: keys.value.kind, publicKey: keys.value.publicKey, testing: testing.value, strict: strict.value })
  : undefined)
</script>

<template>
  <CCard>
    <form class="contents" @submit.prevent="generate">
      <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
        <CInputText v-model:value="domain" :label="t('email.domain')" placeholder="example.com" monospace test-id="domain" :validation="domain && !domainOk ? { isValid: false, status: 'error', message: t('tools.dns-lookup.invalidName') } : undefined" />
        <CInputText v-model:value="selector" :label="t('tools.dkim-generator.selector')" placeholder="s202611" monospace test-id="selector" :validation="!selectorOk ? { isValid: false, status: 'error', message: t('tools.dkim-checker.invalidSelector') } : undefined" />
      </div>
      <NFormItem :label="t('tools.dkim-generator.keyType')">
        <NRadioGroup v-model:value="kind" name="kind">
          <NRadioButton v-for="k in kinds" :key="k" :value="k" :data-testid="`kind-${k}`">
            {{ t(`tools.dkim-generator.kinds.${k}`) }}
          </NRadioButton>
        </NRadioGroup>
      </NFormItem>
      <div class="mb-4 flex flex-wrap gap-x-6 gap-y-1">
        <NCheckbox v-model:checked="testing">
          {{ t('tools.dkim-generator.testing') }}
        </NCheckbox>
        <NCheckbox v-model:checked="strict">
          {{ t('tools.dkim-generator.strict') }}
        </NCheckbox>
      </div>
      <NButton type="primary" attr-type="submit" :loading="busy" :disabled="!domainOk || !selectorOk" data-testid="run">
        {{ keys ? t('tools.dkim-generator.regenerate') : t('tools.dkim-generator.generate') }}
      </NButton>
    </form>
  </CCard>
  <NAlert v-if="failed" type="error" class="mb-4" data-testid="unsupported">
    {{ t('tools.dkim-generator.unsupported') }}
  </NAlert>
  <NAlert v-if="kind === 'ed25519'" type="info" class="mb-4">
    {{ t('tools.dkim-generator.ed25519Hint') }}
  </NAlert>
  <NAlert v-if="kind === 'rsa-1024'" type="warning" class="mb-4">
    {{ t('tools.dkim-generator.shortKey') }}
  </NAlert>

  <template v-if="record && keys">
    <CCard :title="t('tools.dkim-generator.publish')" data-testid="result">
      <RecordList :records="[record]" />
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.dkim-generator.splitHint') }}
      </p>
    </CCard>
    <CCard :title="t('tools.dkim-generator.privateKey')" data-testid="private">
      <NAlert type="warning" class="mb-3">
        {{ t('tools.dkim-generator.privateHint') }}
      </NAlert>
      <NInput :value="keys.privatePem" type="textarea" readonly :autosize="{ minRows: 4, maxRows: 14 }" class="font-mono" :input-props="{ 'aria-label': t('tools.dkim-generator.privateKey') }" />
      <div class="mt-2 flex flex-wrap gap-2">
        <CopyButton :text="keys.privatePem" />
        <NButton secondary size="small" @click="downloadText(`${selector}.${domain}.key`, keys.privatePem, 'application/x-pem-file')">
          {{ t('tools.dkim-generator.download') }}
        </NButton>
      </div>
    </CCard>
  </template>
</template>
