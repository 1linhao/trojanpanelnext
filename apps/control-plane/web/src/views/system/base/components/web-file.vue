<template>
  <div class="app-container">
    <div class="liquid-file-picker">
      <input
        ref="upload"
        class="liquid-file-picker__native"
        type="file"
        accept=".zip"
        @change="handleNativeFile"
      />
      <liquid-button type="primary" @click="$refs.upload.click()">
        {{ $t('config.webFileSelect') }}
      </liquid-button>
      <liquid-button
        @click="submitUpload"
        :disabled="uploading || fileList.length === 0"
      >
        {{ uploading ? $t('config.webFileUploading') : $t('config.webFileBtn') }}
      </liquid-button>
      <span v-if="uploading" class="ui-supporting-text">上传中…</span>
      <span v-else-if="uploadError" class="liquid-file-picker__error">{{ uploadError }}</span>
      <span v-if="fileList.length" class="liquid-file-picker__name">
        {{ fileList[0].name }}
      </span>
      <div class="liquid-file-picker__tip ui-supporting-text">
        {{ $t('config.webFileTip') }}
      </div>
    </div>
  </div>
</template>

<script>
import { uploadWebFile } from '@/api/system'
import { Message } from '@/utils/liquid-feedback'

export default {
  name: 'web-file',
  data() {
    return {
      fileList: [],
      uploading: false,
      uploadError: ''
    }
  },
  methods: {
    handleNativeFile(event) {
      const file = event.target.files[0]
      if (!file || this.beforeUpload(file) === false) {
        event.target.value = ''
        return
      }
      this.fileList = [{ name: file.name, raw: file }]
      this.uploadError = ''
    },
    submitUpload() {
      // WEB-027: busy guard prevents double submit; the draft file stays
      // until the request settles so a failure can be retried directly.
      if (this.uploading || !this.fileList.length) return
      this.uploading = true
      this.uploadError = ''
      const file = this.fileList[0].raw
      this.uploadFile({ file }).finally(() => {
        this.uploading = false
      })
    },
    uploadFile(params) {
      let formData = new FormData()
      formData.append('file', params.file)
      return uploadWebFile(formData).then(() => {
        this.fileList = []
        if (this.$refs.upload) this.$refs.upload.value = ''
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.uploadWebFileSuccess'),
          type: 'success',
          duration: 2000
        })
      }).catch(() => {
        this.uploadError = '上传失败，可直接重试'
      })
    },
    beforeUpload(file) {
      if (!file.name.endsWith('.zip')) {
        Message({
          message: this.$t('confirm.uploadWebFileFormat'),
          type: 'error',
          duration: 5 * 1000
        })
        return false
      }
      if (file.size / 1024 / 1024 > 10) {
        Message({
          message: this.$t('confirm.uploadWebFileSize'),
          type: 'error',
          duration: 5 * 1000
        })
        return false
      }
    }
  }
}
</script>

<style scoped>
.liquid-file-picker {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.liquid-file-picker__native {
  display: none;
}
.liquid-file-picker__name {
  color: var(--ink-2);
  font-size: 12px;
}
.liquid-file-picker__error {
  color: var(--bad-fg);
  font-size: 12px;
}
.liquid-file-picker__tip {
  flex-basis: 100%;
}
</style>
