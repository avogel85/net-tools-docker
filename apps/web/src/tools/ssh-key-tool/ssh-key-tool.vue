<script setup lang="ts">
import type { Fingerprints, SshKey } from './ssh-key-tool.service'
import { NAlert, NFormItem, NInput, NTag } from 'naive-ui'
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import CopyButton from '@/components/CopyButton.vue'
import RecordList from '@/components/RecordList.vue'
import ResultActions from '@/components/ResultActions.vue'
import { useQueryParam } from '@/composables/useQueryParam'
import CCard from '@/ui/c-card.vue'
import CInputText from '@/ui/c-input-text.vue'
import { fingerprints, keyWarnings, parseKeys } from './ssh-key-tool.service'

const { t } = useI18n()
const host = useQueryParam({ name: 'host', defaultValue: '' })
const text = ref('')
const parsed = computed(() => parseKeys(text.value))
const results = shallowRef<{ key: SshKey, fp: Fingerprints }[]>([])
let seq = 0
watch(parsed, async (p) => {
  const id = ++seq
  const out = await Promise.all(p.keys.slice(0, 50).map(async key => ({ key, fp: await fingerprints(key) })))
  if (id === seq)
    results.value = out
}, { immediate: true })

const ownerFor = (k: SshKey) => (host.value.trim() || k.hosts.find(h => h !== '(hashed)' && !/^[\d.:[\]]+$/.test(h)) || 'host.example.com').replace(/^\[([^\]]+)\]:\d+$/, '$1')
const sshfp = computed(() => results.value.flatMap(({ key, fp }) => (fp.sshfp
  ? [
      { name: ownerFor(key), type: 'SSHFP', value: `${fp.sshfp.algorithm} 2 ${fp.sshfp.sha256}` },
      { name: ownerFor(key), type: 'SSHFP', value: `${fp.sshfp.algorithm} 1 ${fp.sshfp.sha1}` },
    ]
  : [])))
const report = () => results.value.map(({ key, fp }) => `${key.type} ${key.bits} ${fp.sha256} ${fp.md5} ${key.comment}`.trim()).join('\n')
</script>

<template>
  <CCard>
    <NFormItem :label="t('tools.ssh-key-tool.keys')">
      <NInput v-model:value="text" type="textarea" :autosize="{ minRows: 4, maxRows: 16 }" class="font-mono" placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA… user@host" :input-props="{ 'aria-label': t('tools.ssh-key-tool.keys'), 'spellcheck': false }" data-testid="keys" />
    </NFormItem>
    <p class="mb-3 mt-0 text-sm">
      {{ t('tools.ssh-key-tool.hint') }}
    </p>
    <CInputText v-model:value="host" :label="t('tools.ssh-key-tool.host')" placeholder="server.example.com" monospace test-id="host" />
  </CCard>
  <NAlert v-if="parsed.errors.length" type="error" class="mb-4" data-testid="parse-errors">
    {{ t('tools.ssh-key-tool.parseErrors', { lines: parsed.errors.join(', ') }) }}
  </NAlert>

  <template v-if="results.length">
    <CCard :title="t('tools.ssh-key-tool.fingerprints')" data-testid="result">
      <div v-for="({ key, fp }, i) in results" :key="i" class="mb-4" data-testid="key">
        <div class="mb-1 flex flex-wrap items-center gap-2">
          <NTag size="small" :bordered="false">
            {{ key.type }}
          </NTag>
          <span class="text-sm">{{ t('tools.ssh-key-tool.bits', { n: key.bits }) }}</span>
          <span v-if="key.comment" class="break-all text-sm font-mono">{{ key.comment }}</span>
          <span v-if="key.hosts.length" class="break-all text-sm font-mono opacity-75">{{ key.hosts.join(', ') }}</span>
        </div>
        <div v-for="f in ([fp.sha256, fp.md5] as const)" :key="f" class="flex flex-wrap items-center gap-2 py-0.5">
          <code class="break-all text-sm font-mono" data-testid="fingerprint">{{ f }}</code>
          <CopyButton :text="f" size="tiny" />
        </div>
        <NAlert v-for="w in keyWarnings(key)" :key="w" type="warning" class="mt-2">
          {{ t(`tools.ssh-key-tool.findings.${w}`) }}
        </NAlert>
      </div>
      <ResultActions :text="report" :json="results.map(r => ({ line: r.key.line, type: r.key.type, bits: r.key.bits, comment: r.key.comment, hosts: r.key.hosts, ...r.fp }))" filename="ssh-fingerprints.json" />
    </CCard>
    <CCard v-if="sshfp.length" :title="t('tools.ssh-key-tool.sshfp')" data-testid="sshfp">
      <RecordList :records="sshfp" />
      <p class="mb-0 mt-3 text-sm">
        {{ t('tools.ssh-key-tool.sshfpHint') }}
      </p>
    </CCard>
  </template>
</template>
