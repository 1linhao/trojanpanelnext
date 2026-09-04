<template>
  <ui-dialog
    append-to-body
    :title="textMap[dialogStatus]"
    :visible="dialogVisible"
    custom-class="liquid-node-server-editor"
    @close="$emit('update:dialogVisible', false)"
  >
    <liquid-form
      ref="dataForm"
      :rules="dialogStatus === 'create' ? createRules : updateRules"
      :model="form"
      class="uniform-dialog-form"
      label-width="132px"
      label-position="left"
    >
      <liquid-form-item :label="$t('table.nodeServerName')" prop="name">
        <liquid-input v-model="form.name" clearable />
      </liquid-form-item>
      <liquid-form-item :label="$t('table.nodeServerIp')" prop="ip">
        <liquid-input v-model="form.ip" clearable />
      </liquid-form-item>
      <liquid-form-item :label="$t('table.nodeServerGrpcPort')" prop="grpcPort">
        <liquid-number-input
          v-model.number="form.grpcPort"
          controls-position="right"
          type="number"
        />
      </liquid-form-item>
      <liquid-form-item
        :label="$t('kernel.tlsServerName')"
        prop="grpcTlsServerName"
      >
        <liquid-input
          v-model="form.grpcTlsServerName"
          :placeholder="$t('kernel.tlsServerNamePlaceholder')"
          clearable
        />
      </liquid-form-item>
      <div class="dialog-section-title">
        <span>{{ $t('traffic.limitSettings') }}</span>
      </div>
      <liquid-form-item :label="$t('traffic.period')">
        <liquid-select v-model="form.trafficPeriod">
          <option :label="$t('traffic.unlimited')" value="none" />
          <option :label="$t('traffic.perDay')" value="day" />
          <option :label="$t('traffic.perMonth')" value="month" />
          <option :label="$t('traffic.perYear')" value="year" />
        </liquid-select>
      </liquid-form-item>
      <liquid-form-item
        v-if="form.trafficPeriod !== 'none'"
        :label="$t('traffic.limitMode')"
      >
        <div class="seg dialog-mode-switch" role="group" :aria-label="$t('traffic.limitMode')">
          <button
            type="button"
            :class="{ on: form.trafficLimitMode === 'combined' }"
            :aria-pressed="String(form.trafficLimitMode === 'combined')"
            @click="form.trafficLimitMode = 'combined'"
          >
            {{ $t('traffic.combined') }}
          </button>
          <button
            type="button"
            :class="{ on: form.trafficLimitMode === 'separate' }"
            :aria-pressed="String(form.trafficLimitMode === 'separate')"
            @click="form.trafficLimitMode = 'separate'"
          >
            {{ $t('traffic.split') }}
          </button>
        </div>
      </liquid-form-item>
      <liquid-form-item
        v-if="
          form.trafficPeriod !== 'none' && form.trafficLimitMode === 'combined'
        "
        :label="$t('traffic.totalLimitGiB')"
      >
        <liquid-number-input
          v-model="form.trafficTotalLimitGiB"
          controls-position="right"
          :min="0"
          :max="8388607"
        />
      </liquid-form-item>
      <template
        v-if="
          form.trafficPeriod !== 'none' && form.trafficLimitMode === 'separate'
        "
      >
        <liquid-form-item :label="$t('traffic.uploadLimitGiB')"
          ><liquid-number-input
            v-model="form.trafficUploadLimitGiB"
            controls-position="right"
            :min="0"
            :max="8388607"
        /></liquid-form-item>
        <liquid-form-item :label="$t('traffic.downloadLimitGiB')"
          ><liquid-number-input
            v-model="form.trafficDownloadLimitGiB"
            controls-position="right"
            :min="0"
            :max="8388607"
        /></liquid-form-item>
      </template>
    </liquid-form>
    <div slot="footer" class="dialog-footer">
      <liquid-button @click="$emit('update:dialogVisible', false)"
        >{{ $t('table.cancel') }}
      </liquid-button>
      <liquid-button
        type="primary"
        @click="dialogStatus === 'create' ? createData() : updateData()"
      >
        {{ $t('table.confirm') }}
      </liquid-button>
    </div>
  </ui-dialog>
</template>

<script>
import { createNodeServer, updateNodeServerById } from '@/api/node-server'

export default {
  name: 'NodeServerForm',
  props: {
    nodeServer: {
      type: Object,
      required: true
    },
    dialogStatus: {
      type: String,
      required: true
    },
    dialogVisible: {
      type: Boolean,
      required: true
    },
    getList: {
      type: Function,
      required: true
    }
  },
  data() {
    return {
      form: Object.assign({}, this.nodeServer),
      textMap: {
        update: this.$t('table.edit'),
        create: this.$t('table.add')
      },
      createRules: {
        ip: [
          {
            required: true,
            message: this.$t('valid.ip'),
            trigger: ['change', 'blur']
          },
          {
            min: 4,
            max: 64,
            message: this.$t('valid.ipRange'),
            trigger: ['change', 'blur']
          }
        ],
        name: [
          {
            required: true,
            message: this.$t('valid.nodeServerName'),
            trigger: ['change', 'blur']
          },
          {
            min: 2,
            max: 20,
            message: this.$t('valid.nodeServerNameRange'),
            trigger: ['change', 'blur']
          }
        ],
        grpcPort: [
          {
            required: true,
            message: this.$t('valid.nodePort'),
            trigger: ['change', 'blur']
          },
          {
            pattern:
              /^([0-9]|[1-9]\d{1,3}|[1-5]\d{4}|6[0-4]\d{4}|65[0-4]\d{2}|655[0-2]\d|6553[0-5])$/,
            message: this.$t('valid.nodePortRange'),
            trigger: ['change', 'blur']
          }
        ],
        grpcTlsServerName: [
          {
            required: true,
            message: this.$t('kernel.tlsServerNameRequired'),
            trigger: ['change', 'blur']
          }
        ]
      },
      updateRules: {
        ip: [
          {
            required: true,
            message: this.$t('valid.ip'),
            trigger: ['change', 'blur']
          },
          {
            min: 4,
            max: 64,
            message: this.$t('valid.ipRange'),
            trigger: ['change', 'blur']
          }
        ],
        name: [
          {
            required: true,
            message: this.$t('valid.nodeServerName'),
            trigger: ['change', 'blur']
          },
          {
            min: 2,
            max: 20,
            message: this.$t('valid.nodeServerNameRange'),
            trigger: ['change', 'blur']
          }
        ],
        grpcPort: [
          {
            required: true,
            message: this.$t('valid.nodePort'),
            trigger: ['change', 'blur']
          },
          {
            pattern:
              /^([0-9]|[1-9]\d{1,3}|[1-5]\d{4}|6[0-4]\d{4}|65[0-4]\d{2}|655[0-2]\d|6553[0-5])$/,
            message: this.$t('valid.nodePortRange'),
            trigger: ['change', 'blur']
          }
        ],
        grpcTlsServerName: [
          {
            min: 4,
            max: 253,
            message: this.$t('kernel.tlsServerNameRequired'),
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  watch: {
    nodeServer: {
      deep: true,
      handler(value) {
        this.form = Object.assign({}, value, {
          trafficTotalLimitGiB:
            (value.trafficTotalLimit || 0) / 1024 / 1024 / 1024,
          trafficUploadLimitGiB:
            (value.trafficUploadLimit || 0) / 1024 / 1024 / 1024,
          trafficDownloadLimitGiB:
            (value.trafficDownloadLimit || 0) / 1024 / 1024 / 1024
        })
      }
    }
  },
  methods: {
    toBytes(value) {
      return Math.round((value || 0) * 1024 * 1024 * 1024)
    },
    payload() {
      const data = Object.assign({}, this.form)
      data.trafficTotalLimit =
        data.trafficPeriod === 'none'
          ? 0
          : this.toBytes(data.trafficTotalLimitGiB)
      data.trafficUploadLimit =
        data.trafficPeriod === 'none'
          ? 0
          : this.toBytes(data.trafficUploadLimitGiB)
      data.trafficDownloadLimit =
        data.trafficPeriod === 'none'
          ? 0
          : this.toBytes(data.trafficDownloadLimitGiB)
      delete data.trafficTotalLimitGiB
      delete data.trafficUploadLimitGiB
      delete data.trafficDownloadLimitGiB
      return data
    },
    clearValidate() {
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    createData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          createNodeServer(this.payload()).then(() => {
            this.getList()
            this.$emit('update:dialogVisible', false)
            this.$notify({
              title: 'Success',
              message: this.$t('confirm.createSuccess'),
              type: 'success',
              duration: 2000
            })
          })
        }
      })
    },
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          const tempData = this.payload()
          updateNodeServerById(tempData).then(() => {
            this.getList()
            this.$emit('update:dialogVisible', false)
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
