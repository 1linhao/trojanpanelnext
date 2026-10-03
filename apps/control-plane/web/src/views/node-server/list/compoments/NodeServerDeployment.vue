<template>
  <ui-dialog
    append-to-body
    :title="$t('nodeDeployment.title')"
    :visible="dialogVisible"
    width="680px"
    custom-class="node-server-deployment-dialog"
    @close="$emit('update:dialogVisible', false)"
  >
    <div class="node-deployment-body" v-liquid-loading="loading">
      <liquid-tabs
        id-prefix="node-deployment"
        :tabs="steps"
        :active-value="activeStep"
        :label="$t('nodeDeployment.title')"
        @change="selectStep"
      />
      <div v-if="loadError" role="alert" class="node-deployment-error">
        <p>{{ loadError }}</p>
        <liquid-button @click="loadDeployment">{{ $t('nodeDeployment.retry') }}</liquid-button>
      </div>
      <template v-else-if="metadata">
        <div
          v-show="activeStep === 'parameters'"
          id="node-deployment-panel-parameters"
          role="tabpanel"
          aria-labelledby="node-deployment-tab-parameters"
        >
          <liquid-form
            ref="deploymentForm"
            :model="form"
            :rules="rules"
            label-position="top"
            class="uniform-dialog-form node-deployment-form"
          >
            <liquid-form-item :label="$t('serverRegistration.serverId')">
              <div class="deployment-control-row">
                <liquid-input :value="metadata.id" readonly />
                <liquid-button icon="document-copy" :aria-label="$t('nodeDeployment.copyId')" @click="copyText(String(metadata.id))" />
              </div>
            </liquid-form-item>
            <liquid-form-item :label="$t('nodeDeployment.nodeAddress')">
              <liquid-input :value="metadata.ip" readonly />
            </liquid-form-item>
            <liquid-form-item :label="$t('kernel.tlsServerName')">
              <liquid-input :value="metadata.grpcTlsServerName" readonly />
            </liquid-form-item>
            <liquid-form-item :label="$t('table.nodeServerGrpcPort')">
              <liquid-input :value="metadata.grpcPort" readonly />
            </liquid-form-item>
            <liquid-form-item :label="$t('nodeDeployment.webHost')" prop="webHost">
              <liquid-input v-model="form.webHost" :placeholder="$t('nodeDeployment.webHostPlaceholder')" clearable />
            </liquid-form-item>
            <liquid-form-item :label="$t('nodeDeployment.database')">
              <liquid-input :value="databaseAddress" readonly />
            </liquid-form-item>
            <liquid-form-item :label="$t('nodeDeployment.redis')">
              <liquid-input :value="redisAddress" readonly />
            </liquid-form-item>
            <liquid-form-item :label="$t('nodeDeployment.certificateMode')" prop="certificateMode">
              <liquid-select v-model="form.certificateMode">
                <option value="caddy" :label="$t('nodeDeployment.caddy')" />
                <option value="external" :label="$t('nodeDeployment.external')" />
              </liquid-select>
            </liquid-form-item>
            <liquid-form-item v-if="form.certificateMode === 'caddy'" :label="$t('nodeDeployment.email')" prop="email">
              <liquid-input v-model="form.email" type="email" placeholder="admin@example.com" clearable />
            </liquid-form-item>
            <template v-else>
              <liquid-form-item :label="$t('nodeDeployment.certificatePath')" prop="certificatePath">
                <liquid-input v-model="form.certificatePath" placeholder="/etc/letsencrypt/live/node.example.com/fullchain.pem" clearable />
              </liquid-form-item>
              <liquid-form-item :label="$t('nodeDeployment.privateKeyPath')" prop="privateKeyPath">
                <liquid-input v-model="form.privateKeyPath" placeholder="/etc/letsencrypt/live/node.example.com/privkey.pem" clearable />
              </liquid-form-item>
            </template>
          </liquid-form>
        </div>
        <div
          v-show="activeStep === 'install'"
          id="node-deployment-panel-install"
          role="tabpanel"
          aria-labelledby="node-deployment-tab-install"
        >
          <p role="note" class="deployment-sensitive-warning">
            <app-icon name="warning-outline" />{{ $t('nodeDeployment.sensitiveWarning') }}
          </p>
          <liquid-form label-position="top" class="uniform-dialog-form">
            <liquid-form-item v-for="requirement in prerequisites" :key="requirement.key" :label="requirement.label">
              <liquid-input :value="requirement.value" type="textarea" :rows="requirement.rows" readonly />
            </liquid-form-item>
            <liquid-form-item :label="$t('nodeDeployment.archive')">
              <div class="deployment-control-row">
                <liquid-input :value="archiveName" readonly />
                <liquid-button type="primary" icon="download" :loading="downloading" @click="downloadArchive">
                  {{ $t('nodeDeployment.download') }}
                </liquid-button>
              </div>
            </liquid-form-item>
            <liquid-form-item v-for="command in installCommands" :key="command.key" :label="command.label">
              <div class="deployment-control-row">
                <liquid-input :value="command.value" :type="command.rows > 1 ? 'textarea' : 'text'" :rows="command.rows" readonly />
                <liquid-button icon="document-copy" :aria-label="$t('nodeDeployment.copyCommand') + ': ' + command.label" @click="copyText(command.value)" />
              </div>
            </liquid-form-item>
          </liquid-form>
        </div>
      </template>
    </div>
    <div slot="footer" class="dialog-footer">
      <liquid-button v-if="metadata" icon="document" @click="openDocumentation">{{ $t('nodeDeployment.documentation') }}</liquid-button>
      <liquid-button @click="$emit('update:dialogVisible', false)">{{ $t('nodeDeployment.close') }}</liquid-button>
      <liquid-button v-if="metadata && activeStep === 'parameters'" type="primary" @click="selectStep('install')">{{ $t('nodeDeployment.next') }}</liquid-button>
      <liquid-button v-else-if="metadata" @click="activeStep = 'parameters'">{{ $t('nodeDeployment.parameters') }}</liquid-button>
    </div>
  </ui-dialog>
</template>

<script>
import copy from 'copy-to-clipboard'
import { nodeServerDeployment, downloadNodeDeployment } from '@/api/node-server'

export default {
  name: 'NodeServerDeployment',
  props: {
    serverId: { type: Number, required: true },
    dialogVisible: { type: Boolean, required: true }
  },
  data() {
    return {
      metadata: null,
      loading: true,
      loadError: '',
      activeStep: 'parameters',
      downloading: false,
      downloaded: false,
      pendingDownloadUrls: [],
      requestSequence: 0,
      form: { webHost: '', email: '', certificateMode: 'caddy', certificatePath: '', privateKeyPath: '' }
    }
  },
  computed: {
    steps() {
      return [
        { value: 'parameters', label: this.$t('nodeDeployment.parameters') },
        { value: 'install', label: this.$t('nodeDeployment.install') }
      ]
    },
    rules() {
      const required = (key) => [{ required: true, message: this.$t(key), trigger: ['change', 'blur'] }]
      return {
        webHost: [...required('nodeDeployment.webHostRequired'), { pattern: /^[A-Za-z0-9][A-Za-z0-9.:-]*$/, message: this.$t('nodeDeployment.hostOnly'), trigger: ['change', 'blur'] }],
        email: this.form.certificateMode === 'caddy' ? [...required('nodeDeployment.emailRequired'), { pattern: /^[^\s@]+@[^\s@]+\.[^\s@]+$/, message: this.$t('nodeDeployment.emailRequired'), trigger: ['change', 'blur'] }] : [],
        certificatePath: this.form.certificateMode === 'external' ? [...required('nodeDeployment.absolutePath'), { pattern: /^\/[^\r\n,]+$/, message: this.$t('nodeDeployment.absolutePath'), trigger: ['change', 'blur'] }] : [],
        privateKeyPath: this.form.certificateMode === 'external' ? [...required('nodeDeployment.absolutePath'), { pattern: /^\/[^\r\n,]+$/, message: this.$t('nodeDeployment.absolutePath'), trigger: ['change', 'blur'] }] : []
      }
    },
    archiveName() { return `tpnext-node-${this.serverId}.tar.gz` },
    databaseAddress() {
      const host = this.metadata.mariadbUsesWebHost === true ? this.form.webHost : this.metadata.mariadbHost
      return `${host}:${this.metadata.mariadbPort}`
    },
    redisAddress() {
      const host = this.metadata.redisUsesWebHost === true ? this.form.webHost : this.metadata.redisHost
      return `${host}:${this.metadata.redisPort}`
    },
    prerequisites() {
      return [
        { key: 'runtime', label: this.$t('nodeDeployment.runtimeRequirements'), value: this.$t('nodeDeployment.runtimeRequirementsValue'), rows: 3 },
        { key: 'tools', label: this.$t('nodeDeployment.baseTools'), value: this.$t('nodeDeployment.baseToolsValue'), rows: 3 }
      ]
    },
    installCommands() {
      return [
        { key: 'bootstrap', label: this.$t('nodeDeployment.bootstrapCommand'), value: 'apt-get update && apt-get install -y bash curl ca-certificates grep coreutils util-linux tar gzip', rows: 3 },
        { key: 'extract', label: this.$t('nodeDeployment.extractCommand'), value: `umask 077 && chmod 600 ${this.archiveName} && tar -xzf ${this.archiveName}`, rows: 2 },
        { key: 'dependencies', label: this.$t('nodeDeployment.dependenciesCommand'), value: 'bash ./tpnext/install-dependencies.sh', rows: 1 },
        { key: 'install', label: this.$t('nodeDeployment.installCommand'), value: 'bash ./tpnext/install-node.sh', rows: 1 }
      ]
    }
  },
  watch: {
    form: { deep: true, handler() { this.downloaded = false } }
  },
  created() { this.loadDeployment() },
  beforeDestroy() {
    this.requestSequence++
    for (const download of this.pendingDownloadUrls) {
      window.clearTimeout(download.timer)
      window.URL.revokeObjectURL(download.url)
    }
    this.pendingDownloadUrls = []
  },
  methods: {
    async loadDeployment() {
      const sequence = ++this.requestSequence
      this.loading = true
      this.loadError = ''
      try {
        const response = await nodeServerDeployment(this.serverId)
        if (sequence !== this.requestSequence) return
        const metadata = response.data
        if (!metadata || metadata.id !== this.serverId || !Number.isSafeInteger(metadata.id) || metadata.id <= 0 ||
            !/^[1-9]\d*\.\d+(?:\.\d+)?(?:-[0-9A-Za-z.-]+)?$/.test(String(metadata.version).replace(/^v/, ''))) {
          throw new Error(this.$t('nodeDeployment.invalidMetadata'))
        }
        this.metadata = metadata
        this.form.webHost = metadata.webHost || ''
      } catch (error) {
        if (sequence === this.requestSequence) this.loadError = error.message || this.$t('nodeDeployment.loadFailed')
      } finally {
        if (sequence === this.requestSequence) this.loading = false
      }
    },
    async selectStep(step) {
      if (!this.metadata || this.loading) return
      if (step === 'install' && !await this.$refs.deploymentForm.validate()) return
      this.activeStep = step
    },
    async downloadArchive() {
      if (this.downloading) return
      const sequence = this.requestSequence
      this.downloading = true
      try {
        if (!await this.$refs.deploymentForm.validate() || sequence !== this.requestSequence) return
        const submittedForm = { ...this.form }
        const request = { id: this.serverId, ...submittedForm }
        if (request.certificateMode === 'caddy') {
          request.certificatePath = ''
          request.privateKeyPath = ''
        } else {
          request.email = ''
        }
        this.downloaded = false
        const response = await downloadNodeDeployment(request)
        if (sequence !== this.requestSequence) return
        const url = window.URL.createObjectURL(response.data)
        const anchor = document.createElement('a')
        try {
          anchor.href = url
          anchor.download = this.archiveName
          document.body.appendChild(anchor)
          anchor.click()
        } finally {
          anchor.remove()
          // Browsers consume Blob URLs asynchronously after anchor.click().
          // Keep the URL alive through that handoff, then release it. Closing
          // the dialog also cleans up every outstanding timer and URL.
          const download = { url, timer: null }
          download.timer = window.setTimeout(() => {
            window.URL.revokeObjectURL(url)
            this.pendingDownloadUrls = this.pendingDownloadUrls.filter((entry) => entry !== download)
          }, 30000)
          this.pendingDownloadUrls.push(download)
        }
        this.downloaded = JSON.stringify(this.form) === JSON.stringify(submittedForm)
        this.$notify({
          title: this.$t('nodeDeployment.downloaded'),
          message: this.$t(this.downloaded ? 'nodeDeployment.downloaded' : 'nodeDeployment.parametersChanged'),
          type: this.downloaded ? 'success' : 'warning',
          duration: this.downloaded ? 4000 : 6000
        })
      } catch (error) {
        if (sequence !== this.requestSequence) return
        this.$notify({ title: this.$t('nodeDeployment.downloadFailed'), message: error.message || this.$t('nodeDeployment.downloadFailed'), type: 'error', duration: 6000 })
      } finally {
        this.downloading = false
      }
    },
    copyText(value) {
      const success = copy(value)
      this.$notify({ title: this.$t(success ? 'nodeDeployment.copySuccess' : 'nodeDeployment.copyFailed'), message: this.$t(success ? 'nodeDeployment.copySuccess' : 'nodeDeployment.copyFailed'), type: success ? 'success' : 'error', duration: 2000 })
    },
    openDocumentation() {
      const version = this.metadata.version.replace(/^v/, '')
      window.open(`https://github.com/1linhao/trojanpanelnext/blob/v${version}/docs/deployment.md#node-deployment-package`, '_blank', 'noopener,noreferrer')
    }
  }
}
</script>

<style scoped>
.node-deployment-body {
  color: var(--ink);
  font-size: 14px;
}
.node-deployment-body .liquid-tabs {
  margin-bottom: 20px;
}
.node-deployment-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 20px;
}
.deployment-control-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}
.deployment-control-row .liquid-input {
  flex: 1;
}
.deployment-control-row .liquid-button {
  flex: 0 0 auto;
  margin-left: 0;
}
.deployment-sensitive-warning {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-weight: 600;
  line-height: 1.6;
}
.node-deployment-error {
  color: var(--bad-fg);
  font-size: 14px;
}
.node-deployment-body ::v-deep .liquid-form-item__label {
  font-size: 14px;
}
@media (max-width: 640px) {
  .node-deployment-form {
    grid-template-columns: minmax(0, 1fr);
  }
  .deployment-control-row {
    flex-wrap: wrap;
  }
  .deployment-control-row .liquid-input {
    flex-basis: 100%;
  }
}
</style>
