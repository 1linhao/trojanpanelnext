<template>
  <ui-dialog
    append-to-body
    :title="$t('table.add').toString()"
    :visible="dialogVisibleProps"
    @close="$emit('update:dialogVisibleProps', false)"
    width="30%"
  >
    <liquid-form
      ref="dataForm"
      :rules="createRules"
      :model="temp"
      label-position="left"
    >
      <liquid-form-item label="name" prop="name">
        <liquid-input v-model="temp.name" clearable />
      </liquid-form-item>
      <liquid-form-item label="alpn" prop="alpn">
        <liquid-input v-model="temp.alpn" clearable />
      </liquid-form-item>
      <liquid-form-item label="path" prop="path">
        <liquid-input v-model="temp.path" clearable />
      </liquid-form-item>
      <liquid-form-item label="dest" prop="dest">
        <liquid-input v-model="temp.dest" clearable />
      </liquid-form-item>
      <liquid-form-item label="xver" prop="xver">
        <liquid-number-input
          v-model.number="temp.xver"
          controls-position="right"
          type="number"
        />
      </liquid-form-item>
    </liquid-form>
    <div slot="footer" class="dialog-footer">
      <liquid-button @click="$emit('update:dialogVisibleProps', false)"
        >{{ $t('table.cancel') }}
      </liquid-button>
      <liquid-button type="primary" @click="createData()">
        {{ $t('table.confirm') }}
      </liquid-button>
    </div>
  </ui-dialog>
</template>

<script>
export default {
  name: 'FallbackForm',
  props: {
    createFallbackProps: {
      type: Function,
      required: true
    },
    dialogVisibleProps: {
      type: Boolean,
      required: true
    }
  },
  data() {
    const pathPrefixValidate = (rule, value, callback) => {
      if (this.temp.path && !this.temp.path.startsWith('/')) {
        callback(new Error(this.$t('valid.xrayFallbackPathPrefix')))
      } else {
        callback()
      }
    }
    const xverValidate = (rule, value, callback) => {
      if (
        this.temp.xver !== 0 &&
        this.temp.xver !== 1 &&
        this.temp.xver !== 2
      ) {
        callback(new Error(this.$t('valid.xrayFallbackXver')))
      } else {
        callback()
      }
    }
    return {
      temp: {
        name: '',
        alpn: '',
        path: undefined,
        dest: '80',
        xver: 0
      },
      createRules: {
        path: [
          {
            validator: pathPrefixValidate,
            trigger: ['change', 'blur']
          }
        ],
        dest: [
          {
            required: true,
            message: this.$t('valid.xrayFallbackDest'),
            trigger: ['change', 'blur']
          }
        ],
        xver: [
          {
            validator: xverValidate,
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  methods: {
    resetTemp() {
      this.temp = {
        name: '',
        alpn: '',
        path: undefined,
        dest: '80',
        xver: 0
      }
    },
    createData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          let tempData = {
            dest: this.temp.dest
          }
          if (this.temp.name) {
            tempData.name = this.temp.name
          }
          if (this.temp.alpn) {
            tempData.alpn = this.temp.alpn
          }
          if (this.temp.xver) {
            tempData.xver = this.temp.xver
          }
          // path以/开头否则不传改字段
          if (this.temp.path && this.temp.path.startsWith('/')) {
            tempData.path = this.temp.path
          }
          this.createFallbackProps(tempData)
          this.$emit('update:dialogVisibleProps', false)
        }
      })
    }
  }
}
</script>

<style scoped></style>
