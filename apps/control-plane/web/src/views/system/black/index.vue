<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="blacklist-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" /><input
            v-model="listQuery.ip"
            placeholder="按 IP 地址搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <div class="spacer"></div>
        <button class="cap primary small" type="button" @click="handleCreate">
          <app-icon name="plus" />添加黑名单
        </button>
      </div>
    </ui-panel>
    <ui-panel motion-key="blacklist-list">
      <div class="tbl-wrap" v-liquid-loading="listLoading">
        <table class="tbl">
          <thead>
            <tr>
              <th>IP 地址</th>
              <th>创建时间</th>
              <th class="table-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!listLoading && listError" class="tbl-empty">
              <td colspan="3">{{ listError }}</td>
            </tr>
            <tr v-else-if="!listLoading && !list.length" class="tbl-empty">
              <td colspan="3">暂无数据</td>
            </tr>
            <tr v-for="(row, index) in list" :key="row.id || row.ip">
              <td class="primary-cell">
                <strong class="mono">{{ row.ip }}</strong
                ><small>已阻止访问</small>
              </td>
              <td class="muted">
                {{ timeStampToDate(row.createTime, false) }}
              </td>
              <td>
                <div class="row-actions">
                  <button
                    class="icon-btn danger"
                    type="button"
                    title="删除"
                    @click="handleDelete(row, index)"
                  >
                    <app-icon name="delete" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <pagination
        v-if="total > 0"
        :total="total"
        :page.sync="listQuery.pageNum"
        :limit.sync="listQuery.pageSize"
        @pagination="getList"
      />
    </ui-panel>
    <ui-dialog
      append-to-body
      :title="$t('table.add')"
      :visible.sync="dialogFormVisible"
    >
      <liquid-form
        ref="dataForm"
        :rules="createRules"
        :model="temp"
        label-position="left"
      >
        <liquid-form-item :label="$t('table.blackListIp')" prop="ip">
          <liquid-input v-model="temp.ip" clearable />
        </liquid-form-item>
      </liquid-form>
      <div slot="footer" class="dialog-footer">
        <liquid-button @click="dialogFormVisible = false"
          >{{ $t('table.cancel') }}
        </liquid-button>
        <liquid-button type="primary" @click="createData()">
          {{ $t('table.confirm') }}
        </liquid-button>
      </div>
    </ui-dialog>
  </div>
</template>

<script>
import { timeStampToDate } from '@/utils'
import Pagination from '@/components/Pagination'
import { MessageBox } from '@/utils/liquid-feedback'
import {
  createBlackList,
  deleteBlackListByIp,
  selectBlackListPage
} from '@/api/black-list'
import checkPermission from '@/utils/permission' // 权限判断指令
import latestListRequest from '@/mixins/latest-list-request'

export default {
  mixins: [latestListRequest],
  name: 'BlacklistPage',
  components: { Pagination },
  data() {
    const validateIp = (rule, value, callback) => {
      if (this.temp.ip === '127.0.0.1') {
        callback(new Error('IP cannot be 127.0.0.1'))
      } else {
        callback()
      }
    }
    return {
      tableKey: 0,
      listLoading: true,
      listError: '',
      list: null,
      total: 0,
      listQuery: {
        pageNum: 1,
        pageSize: 20,
        ip: undefined
      },
      temp: {
        id: undefined,
        ip: '',
        createTime: new Date()
      },
      dialogFormVisible: false,
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
          },
          {
            validator: validateIp,
            trigger: ['change', 'blur']
          }
        ]
      }
    }
  },
  created() {
    this.getList()
  },
  methods: {
    checkPermission,
    timeStampToDate,
    getList() {
      const request = this.beginListRequest()
      return selectBlackListPage(this.listQuery).then((response) => {
        if (!this.ownsListRequest(request)) return
        this.list = response.data.blackLists
        this.total = response.data.total
      }).catch(() => {
        if (!this.ownsListRequest(request)) return
        this.list = []
        this.total = 0
        this.listError = '请求失败，请重试'
      }).finally(() => this.finishListRequest(request))
    },
    resetTemp() {
      this.temp = {
        id: undefined,
        ip: '',
        createTime: new Date()
      }
    },
    handleFilter() {
      this.listQuery.pageNum = 1
      this.getList()
    },
    handleCreate() {
      this.resetTemp()
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleDelete(row, index) {
      MessageBox.confirm(
        this.$t('confirm.deleteBlack').toString(),
        this.$t('confirm.warn').toString(),
        {
          confirmButtonText: this.$t('confirm.yes'),
          cancelButtonText: this.$t('confirm.cancel'),
          type: 'warning'
        }
      ).then(() => {
        const tempData = Object.assign({}, row)
        deleteBlackListByIp(tempData).then(() => {
          this.list.splice(index, 1)
          this.$notify({
            title: 'Success',
            message: this.$t('confirm.deleteSuccess'),
            type: 'success',
            duration: 2000
          })
        })
      })
    },
    createData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          createBlackList(this.temp).then(() => {
            this.getList()
            this.dialogFormVisible = false
            this.$notify({
              title: 'Success',
              message: this.$t('confirm.createSuccess'),
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

<style scoped>
.liquid-button {
  margin-left: 10px;
}
</style>
