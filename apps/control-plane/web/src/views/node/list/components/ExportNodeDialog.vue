<template>
  <ui-dialog
    append-to-body
    :title="$t('exportNode.title').toString()"
    :visible.sync="dialogVisible"
    width="92%"
    custom-class="export-node-dialog"
    @open="loadOptions"
    @closed="reset"
  >
    <liquid-tabs
      id-prefix="export-client"
      :tabs="clientTabs"
      :active-value="activeClient"
      @change="selectClient"
      :label="$t('exportNode.title').toString()"
    />

    <div :id="`export-client-panel-${activeClient}`" role="tabpanel" :aria-labelledby="`export-client-tab-${activeClient}`">
    <liquid-form label-position="top">
      <liquid-form-item :label="$t('exportNode.template').toString()">
        <liquid-select v-model="selectedTemplate">
          <option
            v-for="template in activeOption.templates"
            :key="template.id"
            :label="template.name"
            :value="template.id"
          />
        </liquid-select>
      </liquid-form-item>
      <liquid-form-item
        class="format-actions"
        :label="$t('exportNode.format').toString()"
      >
        <liquid-button
          v-for="format in activeOption.formats"
          :key="format"
          :icon="formatIcon(format)"
          :loading="loadingFormat === format"
          @click="handleExport(format)"
        >
          {{ $t(`exportNode.${format}`) }}
        </liquid-button>
      </liquid-form-item>
    </liquid-form>

    <div v-if="qrCodeSrc" class="qrcode">
      <img :src="qrCodeSrc" :alt="$t('exportNode.qrcode').toString()" />
    </div>
    </div>
  </ui-dialog>
</template>

<script>
import copy from 'copy-to-clipboard'
import { Message } from '@/utils/liquid-feedback'
import { exportOptions, exportQRCode, exportSubscribe } from '@/api/account'

export default {
  name: 'ExportNodeDialog',
  props: {
    dialogVisibleProps: {
      type: Boolean,
      required: true
    }
  },
  data() {
    return {
      options: [],
      activeClient: 'sing-box',
      selectedTemplate: '',
      loadingFormat: '',
      qrCodeSrc: ''
    }
  },
  computed: {
    clientTabs() {
      return this.options.map((option) => ({ value: option.id, label: option.name }))
    },
    dialogVisible: {
      get() {
        return this.dialogVisibleProps
      },
      set(value) {
        this.$emit('update:dialogVisibleProps', value)
      }
    },
    activeOption() {
      return (
        this.options.find((item) => item.id === this.activeClient) || {
          templates: [],
          formats: []
        }
      )
    }
  },
  methods: {
    selectClient(client) {
      this.activeClient = client
      this.selectDefaultTemplate()
    },
    loadOptions() {
      exportOptions().then((response) => {
        this.options = response.data || []
        if (!this.options.some((item) => item.id === this.activeClient)) {
          this.activeClient = this.options.length ? this.options[0].id : ''
        }
        this.selectDefaultTemplate()
      })
    },
    selectDefaultTemplate() {
      const templates = this.activeOption.templates || []
      this.selectedTemplate = templates.length ? templates[0].id : ''
      this.qrCodeSrc = ''
    },
    formatIcon(format) {
      return {
        link: 'document-copy',
        file: 'download',
        qrcode: 'full-screen'
      }[format]
    },
    absoluteUrl(path) {
      return new URL(path, window.location.origin).toString()
    },
    handleExport(format) {
      if (!this.activeClient || !this.selectedTemplate) return
      this.loadingFormat = format
      const params = {
        client: this.activeClient,
        template: this.selectedTemplate
      }
      if (format === 'qrcode') {
        exportQRCode(params)
          .then((response) => {
            this.qrCodeSrc = `data:image/png;base64,${response.data}`
          })
          .finally(() => {
            this.loadingFormat = ''
          })
        return
      }
      exportSubscribe(params)
        .then((response) => {
          const url = this.absoluteUrl(response.data)
          if (format === 'link') {
            const success = copy(url)
            Message({
              showClose: true,
              message: this.$t(
                success ? 'confirm.urlCopySuccess' : 'confirm.urlCopyFail'
              ).toString(),
              type: success ? 'success' : 'error'
            })
            return
          }
          const anchor = document.createElement('a')
          anchor.href = url
          anchor.rel = 'noopener'
          document.body.appendChild(anchor)
          anchor.click()
          anchor.remove()
        })
        .finally(() => {
          this.loadingFormat = ''
        })
    },
    reset() {
      this.loadingFormat = ''
      this.qrCodeSrc = ''
    }
  }
}
</script>

<style scoped>
.qrcode {
  display: flex;
  justify-content: center;
  padding-top: 8px;
}

.qrcode img {
  width: 256px;
  height: 256px;
}

.format-actions ::v-deep .liquid-form-item__content {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.format-actions .liquid-button {
  margin-left: 0;
}

@media (max-width: 640px) {
  .qrcode img {
    width: min(256px, 100%);
    height: auto;
    aspect-ratio: 1;
  }
}
</style>

<style>
.export-node-dialog {
  max-width: 560px;
}
</style>
