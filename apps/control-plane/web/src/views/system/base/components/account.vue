<template>
  <div class="app-container">
    <liquid-form
      ref="dataForm"
      :rules="updateRules"
      :model="systemConfig"
      label-position="left"
    >
      <liquid-form-item :label="$t('config.registerEnable')" prop="registerEnable">
        <liquid-switch
          v-model="systemConfig.registerEnable"
          :active-value="1"
          :inactive-value="0"
          class="ml-2"
        />
      </liquid-form-item>
      <liquid-form-item :label="$t('config.registerQuota')" prop="registerQuota">
        <liquid-number-input
          v-model.number="systemConfig.registerQuota"
          controls-position="right"
          type="number"
          :disabled="registerDisable"
        />
      </liquid-form-item>
      <liquid-form-item
        :label="$t('config.registerExpireDays')"
        prop="registerExpireDays"
      >
        <liquid-number-input
          v-model.number="systemConfig.registerExpireDays"
          controls-position="right"
          type="number"
          :disabled="registerDisable"
        />
      </liquid-form-item>
      <liquid-form-item
        :label="$t('config.resetDownloadAndUploadMonth')"
        prop="resetDownloadAndUploadMonth"
      >
        <liquid-switch
          v-model="systemConfig.resetDownloadAndUploadMonth"
          :active-value="1"
          :inactive-value="0"
          class="ml-2"
        />
      </liquid-form-item>
      <liquid-form-item
        :label="$t('config.trafficRankEnable')"
        prop="trafficRankEnable"
      >
        <liquid-switch
          v-model="systemConfig.trafficRankEnable"
          :active-value="1"
          :inactive-value="0"
          class="ml-2"
        />
      </liquid-form-item>
      <liquid-form-item :label="$t('config.captchaEnable')" prop="captchaEnable">
        <liquid-switch
          v-model="systemConfig.captchaEnable"
          :active-value="1"
          :inactive-value="0"
          class="ml-2"
        />
      </liquid-form-item>
      <liquid-form-item>
        <liquid-button type="primary" @click="updateData()"
          >{{ $t('table.confirm') }}
        </liquid-button>
      </liquid-form-item>
    </liquid-form>
  </div>
</template>

<script>
import { updateSystemById } from '@/api/system'

export default {
  name: 'AccountSettingsForm',
  inject: {
    systemConfig: {
      from: 'systemConfigModel'
    }
  },
  computed: {
    registerDisable: function () {
      return this.systemConfig.registerEnable === 0
    }
  },
  data() {
    return {
      updateRules: {
        registerEnable: [
          {
            required: true,
            message: this.$t('valid.registerEnable'),
            trigger: ['change', 'blur']
          }
        ],
        registerQuota: [
          {
            required: true,
            message: this.$t('valid.registerQuota'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: 0,
            max: 1024000,
            message: this.$t('valid.registerQuotaRange'),
            trigger: ['change', 'blur']
          }
        ],
        registerExpireDays: [
          {
            required: true,
            message: this.$t('valid.registerExpireDays'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: 0,
            max: 365,
            message: this.$t('valid.registerExpireDaysRange'),
            trigger: ['change', 'blur']
          }
        ],
        resetDownloadAndUploadMonth: [
          {
            required: true,
            message: this.$t('valid.resetDownloadAndUploadMonth'),
            trigger: ['change', 'blur']
          }
        ],
        trafficRankEnable: [
          {
            required: true,
            message: this.$t('valid.trafficRankEnable'),
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  methods: {
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          const tempData = Object.assign({}, this.systemConfig)
          updateSystemById(tempData).then(() => {
            this.$nextTick(() => {
              this.$refs['dataForm'].clearValidate()
            })
            this.$notify({
              title: 'Success',
              message: this.$t('confirm.modifySuccess'),
              type: 'success',
              duration: 2000
            })
          })
        }
      })
    }
  }
}
</script>

<style scoped></style>
