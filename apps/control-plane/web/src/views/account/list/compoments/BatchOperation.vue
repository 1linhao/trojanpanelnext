<template>
  <ui-dialog
    append-to-body
    :title="$t('table.createBatch').toString()"
    :visible="dialogFormVisibleProps"
    @close="$emit('update:dialogFormVisibleProps', false)"
  >
    <liquid-form
      ref="dataForm"
      :rules="createRules"
      :model="temp"
      label-position="left"
    >
      <liquid-form-item :label="$t('table.num').toString()" prop="num">
        <liquid-number-input
          v-model.number="temp.num"
          controls-position="right"
          type="number"
        />
      </liquid-form-item>
      <liquid-form-item
        :label="$t('table.presetQuota').toString()"
        prop="presetQuota"
      >
        <liquid-number-input
          v-model.number="temp.presetQuota"
          controls-position="right"
          type="number"
        />
      </liquid-form-item>
      <liquid-form-item
        :label="$t('table.presetExpire').toString()"
        prop="presetExpire"
      >
        <liquid-number-input
          v-model.number="temp.presetExpire"
          controls-position="right"
          type="number"
        />
      </liquid-form-item>
    </liquid-form>
    <div slot="footer" class="dialog-footer">
      <liquid-button @click="$emit('update:dialogFormVisibleProps', false)"
        >{{ $t('table.cancel') }}
      </liquid-button>
      <liquid-button type="primary" :disabled="creating" @click="createAccountBatch">
        {{ $t('table.confirm') }}
      </liquid-button>
    </div>
  </ui-dialog>
</template>

<script>
import { createAccountBatch } from '@/api/account'

export default {
  name: 'BatchOperation',
  props: {
    dialogFormVisibleProps: {
      type: Boolean,
      required: true
    },
    getListProps: {
      type: Function,
      required: true
    }
  },
  data() {
    return {
      creating: false,
      temp: {
        num: 5,
        presetQuota: 1024,
        presetExpire: 7
      },
      createRules: {
        num: [
          {
            required: true,
            message: this.$t('valid.createBatchNum'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: 5,
            max: 500,
            message: this.$t('valid.createBatchNumRange'),
            trigger: ['change', 'blur']
          }
        ],
        presetQuota: [
          {
            required: true,
            message: this.$t('valid.createBatchPresetQuota'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: -1,
            max: 1024000,
            message: this.$t('valid.createBatchPresetQuotaRange'),
            trigger: ['change', 'blur']
          }
        ],
        presetExpire: [
          {
            required: true,
            message: this.$t('valid.createBatchPresetExpire'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: 1,
            max: 365,
            message: this.$t('valid.createBatchPresetExpireRange'),
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  methods: {
    clearValidate() {
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    async createAccountBatch() {
      if (this.creating) return
      this.creating = true
      try {
        if (!(await this.$refs.dataForm.validate())) return
        await createAccountBatch(this.temp)
        this.getListProps()
        this.$emit('update:dialogFormVisibleProps', false)
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.createSuccess').toString(),
          type: 'success',
          duration: 2000
        })
      } finally {
        this.creating = false
      }
    }
  }
}
</script>

<style scoped></style>
