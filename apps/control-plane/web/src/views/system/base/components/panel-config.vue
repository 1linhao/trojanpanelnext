<template>
  <div class="app-container panel-config">
    <liquid-form
      ref="dataForm"
      :rules="updateRules"
      :model="systemConfig"
      label-position="top"
    >
      <div class="panel-config__fields">
        <liquid-form-item :label="$t('config.systemLogo')">
          <upload-logo />
        </liquid-form-item>
        <liquid-form-item :label="$t('config.systemName')" prop="systemName">
          <liquid-input v-model="systemConfig.systemName" clearable />
        </liquid-form-item>
      </div>

      <liquid-form-item class="panel-config__actions">
        <liquid-button
          type="primary"
          icon="check"
          @click="updateData"
        >
          {{ $t('table.confirm') }}
        </liquid-button>
      </liquid-form-item>
    </liquid-form>
  </div>
</template>

<script>
import { updateSystemById } from '@/api/system'
import UploadLogo from '@/components/UploadLogo'
import { updatePanelBranding } from '@/utils/panel-branding'

export default {
  name: 'PanelConfigForm',
  components: { UploadLogo },
  inject: {
    systemConfig: {
      from: 'systemConfigModel'
    }
  },
  data() {
    return {
      updateRules: {
        systemName: [
          {
            required: true,
            message: this.$t('valid.systemName'),
            trigger: ['change', 'blur']
          },
          {
            min: 2,
            max: 32,
            message: this.$t('valid.systemNameRange'),
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  methods: {
    updateData() {
      this.$refs.dataForm.validate((valid) => {
        if (!valid) return
        const savedConfig = Object.assign({}, this.systemConfig)
        updateSystemById(savedConfig).then(() => {
          updatePanelBranding(savedConfig)
          this.$nextTick(() => this.$refs.dataForm.clearValidate())
          this.$notify({
            title: 'Success',
            message: this.$t('confirm.modifySuccess'),
            type: 'success',
            duration: 2000
          })
        })
      })
    }
  }
}
</script>

<style scoped>
.panel-config__fields {
  display: grid;
  grid-template-columns: minmax(160px, 240px) minmax(240px, 1fr);
  gap: 24px;
}

.panel-config__actions {
  margin-top: 20px;
}

@media (max-width: 720px) {
  .panel-config__fields {
    grid-template-columns: 1fr;
    gap: 0;
  }
}
</style>
