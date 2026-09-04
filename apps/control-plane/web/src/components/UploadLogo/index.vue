<template>
  <div class="liquid-logo-picker">
    <panel-logo class="liquid-logo-picker__preview" />
    <input
      ref="upload"
      class="liquid-logo-picker__native"
      type="file"
      accept="image/png"
      :disabled="uploading"
      @change="handleNativeFile"
    />
    <liquid-button icon="upload2" :loading="uploading" @click="$refs.upload.click()">
      {{ uploading ? '上传中…' : '更换 Logo' }}
    </liquid-button>
    <div class="liquid-logo-picker__tip ui-supporting-text">
      {{ $t('config.imageFileTip') }}
    </div>
  </div>
</template>

<script>
import { Message } from '@/utils/liquid-feedback'
import { uploadLogo } from '@/api/system'
import PanelLogo from '@/components/PanelLogo'
import { refreshPanelLogo } from '@/utils/panel-branding'

export default {
  name: 'LogoUploader',
  components: { PanelLogo },
  data: () => ({ uploading: false }),
  methods: {
    async handleNativeFile(event) {
      const file = event.target.files[0]
      event.target.value = ''
      if (!file || this.uploading || !this.beforeUpload(file)) return
      this.uploading = true
      try {
        const formData = new FormData()
        formData.append('file', file)
        await uploadLogo(formData)
        refreshPanelLogo()
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.modifySuccess'),
          type: 'success',
          duration: 2000
        })
      } catch (error) {
        // Request feedback handles failures; retain the last saved Logo.
      } finally {
        this.uploading = false
      }
    },
    beforeUpload(file) {
      const isPNG = file.type === 'image/png'
      const isUnderLimit = file.size / 1024 / 1024 < 3

      if (!isPNG) {
        Message({
          message: this.$t('confirm.uploadWebFileFormat'),
          type: 'error',
          duration: 5 * 1000
        })
      }
      if (!isUnderLimit) {
        Message({
          message: this.$t('confirm.uploadWebFileSize'),
          type: 'error',
          duration: 5 * 1000
        })
      }
      return isPNG && isUnderLimit
    }
  }
}
</script>
<style scoped>
.liquid-logo-picker {
  display: grid;
  gap: 8px;
  justify-items: start;
}
.liquid-logo-picker__native {
  display: none;
}
.liquid-logo-picker__preview {
  width: 112px;
  height: 112px;
  border: 1px solid var(--rim);
  border-radius: 20px;
  background: var(--control-fill);
  box-shadow: inset 0 1px 0 var(--spec-soft), var(--shadow-soft);
  color: var(--accent);
  font-size: 36px;
}
</style>
