<script setup lang="ts">
import type { TsigAlgorithm } from './nsupdate-builder.service'
import { NAlert, NButton, NCheckbox, NFormItem, NInput, NTabPane, NTabs } from 'naive-ui'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import CSelect from '@/ui/c-select.vue'
import { downloadText } from '@/utils/download'
import { buildScript, generateSecret, isValidName, isValidSecret, keyConfigs, parseOperations, TSIG_ALGORITHMS } from './nsupdate-builder.service'

const { t } = useI18n()
const keyName = useQueryParam({ name: 'key', defaultValue: 'ddns-key' })
const algorithm = useQueryParam({ name: 'alg', defaultValue: 'hmac-sha256' })
// The secret is never put in the URL or browser storage.
const secret = ref('')
const server = useQueryParam({ name: 'server', defaultValue: '' })
const port = useQueryParam({ name: 'port', defaultValue: 53 })
const zone = useQueryParam({ name: 'zone', defaultValue: '' })
const ops = useQueryParam({ name: 'ops', defaultValue: 'replace www 300 A 192.0.2.10' })
const showAnswer = useQueryParam({ name: 'show', defaultValue: true })

const algOptions = TSIG_ALGORITHMS.map(a => ({ value: a, label: a === 'hmac-md5' || a === 'hmac-sha1' ? `${a} (${t('tools.nsupdate-builder.legacy')})` : a }))
const keyOk = computed(() => isValidName(keyName.value) && keyName.value !== '@')
const secretOk = computed(() => !secret.value || isValidSecret(secret.value))
const configs = computed(() => (keyOk.value && secret.value && secretOk.value ? keyConfigs({ name: keyName.value, algorithm: algorithm.value as TsigAlgorithm, secret: secret.value }) : undefined))
const tab = ref<'bind' | 'knot' | 'powerdns' | 'cli'>('bind')

const parsed = computed(() => parseOperations(ops.value))
const errors = computed(() => parsed.value.filter(p => p.error))
const portText = computed({
  get: () => String(port.value),
  set: (v: string) => {
    port.value = /^\d{1,5}$/.test(v.trim()) ? Number(v) : 0
  },
})
const portOk = computed(() => port.value >= 1 && port.value <= 65535)
const script = computed(() => buildScript({ server: server.value.trim(), port: port.value, zone: zone.value.trim(), ops: parsed.value.flatMap(p => (p.op ? [p.op] : [])), showAnswer: showAnswer.value }))
const command = computed(() => `nsupdate${secret.value ? ` -k ${keyName.value.replace(/\.$/, '')}.key` : ''} update.txt`)
</script>

<template>
  <CCard :title="t('tools.nsupdate-builder.tsig')">
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-2">
      <CInputText v-model:value="keyName" :label="t('tools.nsupdate-builder.keyName')" monospace test-id="key-name" :validation="keyOk ? undefined : { isValid: false, status: 'error', message: t('tools.nsupdate-builder.invalidKeyName') }" />
      <CSelect v-model:value="algorithm" :label="t('tools.nsupdate-builder.algorithm')" :options="algOptions" />
    </div>
    <NFormItem :label="t('tools.nsupdate-builder.secret')" :validation-status="secretOk ? undefined : 'error'" :feedback="secretOk ? undefined : t('tools.nsupdate-builder.invalidSecret')">
      <div class="w-full flex flex-wrap gap-2">
        <NInput v-model:value="secret" class="min-w-60 flex-1 font-mono" :placeholder="t('tools.nsupdate-builder.secretPlaceholder')" :input-props="{ 'aria-label': t('tools.nsupdate-builder.secret'), 'spellcheck': false }" data-testid="secret" />
        <NButton secondary data-testid="generate" @click="secret = generateSecret(algorithm as TsigAlgorithm)">
          {{ t('tools.nsupdate-builder.generate') }}
        </NButton>
      </div>
    </NFormItem>
    <template v-if="configs">
      <NTabs v-model:value="tab" type="line" animated>
        <NTabPane v-for="k in (['bind', 'knot', 'powerdns', 'cli'] as const)" :key="k" :name="k" :tab="t(`tools.nsupdate-builder.formats.${k}`)">
          <pre class="m-0 mb-2 overflow-x-auto text-xs font-mono" :data-testid="`config-${k}`">{{ configs[k] }}</pre>
          <div class="flex flex-wrap gap-2">
            <CopyButton :text="configs[k]" />
            <NButton v-if="k === 'bind'" secondary size="small" @click="downloadText(`${keyName.replace(/\.$/, '')}.key`, configs.bind, 'text/plain')">
              {{ t('tools.nsupdate-builder.downloadKey') }}
            </NButton>
          </div>
        </NTabPane>
      </NTabs>
      <NAlert type="warning" class="mt-3">
        {{ t('tools.nsupdate-builder.secretHint') }}
      </NAlert>
    </template>
  </CCard>

  <CCard :title="t('tools.nsupdate-builder.updates')">
    <div class="grid grid-cols-1 gap-x-4 md:grid-cols-[1fr_8rem_1fr]">
      <CInputText v-model:value="server" :label="t('tools.nsupdate-builder.server')" placeholder="ns1.example.com" monospace test-id="server" />
      <CInputText v-model:value="portText" :label="t('tools.nsupdate-builder.port')" placeholder="53" monospace test-id="port" :validation="portOk ? undefined : { isValid: false, status: 'error', message: t('tools.nsupdate-builder.invalidPort') }" />
      <CInputText v-model:value="zone" :label="t('tools.nsupdate-builder.zone')" placeholder="example.com" monospace test-id="zone" />
    </div>
    <p class="mb-2 mt-0 text-sm">
      {{ t('tools.nsupdate-builder.syntax') }}
    </p>
    <NFormItem :label="t('tools.nsupdate-builder.operations')" :validation-status="errors.length ? 'error' : undefined" :feedback="errors.length ? errors.map(e => t('tools.nsupdate-builder.lineError', { line: e.line, error: t(`tools.nsupdate-builder.errors.${e.error}`) })).join(' · ') : undefined">
      <NInput v-model:value="ops" type="textarea" :autosize="{ minRows: 4, maxRows: 16 }" class="font-mono" :input-props="{ 'aria-label': t('tools.nsupdate-builder.operations'), 'spellcheck': false }" data-testid="ops" />
    </NFormItem>
    <NCheckbox v-model:checked="showAnswer" class="mb-4">
      {{ t('tools.nsupdate-builder.showAnswer') }}
    </NCheckbox>
    <h3 class="mb-2 mt-0 text-sm font-semibold">
      {{ t('tools.nsupdate-builder.script') }}
    </h3>
    <pre class="m-0 mb-2 overflow-x-auto text-xs font-mono" data-testid="script">{{ script }}</pre>
    <div class="mb-3 flex flex-wrap gap-2">
      <CopyButton :text="script" />
      <NButton secondary size="small" @click="downloadText('update.txt', script, 'text/plain')">
        {{ t('tools.nsupdate-builder.downloadScript') }}
      </NButton>
    </div>
    <p class="m-0 text-sm">
      {{ t('tools.nsupdate-builder.run') }} <code class="font-mono">{{ command }}</code>
    </p>
  </CCard>
</template>
