<template>
  <div class="prototype-page grid">
    <ui-panel motion-key="account-filters">
      <div class="toolbar">
        <div class="search-box">
          <app-icon name="search" />
          <input
            v-model="listQuery.username"
            placeholder="按用户名搜索"
            @keyup.enter="handleFilter"
          />
        </div>
        <liquid-select
          v-model="listQuery.deleted"
          clearable
          placeholder="账号状态"
          class="prototype-select"
        >
          <option
            v-for="item in deletedList"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </liquid-select>
        <div class="spacer"></div>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleCreateBatch"
        >
          <app-icon name="user-plus" />批量创建
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleImport"
        >
          <app-icon name="import" />导入
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="handleExport"
        >
          <app-icon name="export" />导出
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap small"
          type="button"
          @click="exportAccountUnused"
        >
          <app-icon name="export" />导出未使用
        </button>
        <button
          v-if="checkPermission(['sysadmin'])"
          class="cap primary small"
          type="button"
          @click="handleCreate"
        >
          <app-icon name="plus" />新建账号
        </button>
      </div>
    </ui-panel>

    <ui-panel motion-key="account-list">
      <div class="tbl-wrap" v-liquid-loading="listLoading">
        <table class="tbl">
          <thead>
            <tr>
              <th>账号</th>
              <th>角色</th>
              <th class="traffic-reset-column">
                <div class="traffic-reset-column-head">
                  <span>流量（已用 / 配额）</span>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="cap small traffic-reset-button"
                    type="button"
                    title="重置所有用户流量统计"
                    aria-label="重置所有用户流量统计"
                    @click="handleResetAllAccountTraffic"
                  >
                    <app-icon name="refresh" />重置全部
                  </button>
                </div>
              </th>
              <th class="account-expiry-column">到期时间</th>
              <th>最近登录</th>
              <th>状态</th>
              <th class="table-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!listLoading && listError" class="tbl-empty">
              <td colspan="8">{{ listError }}</td>
            </tr>
            <tr v-else-if="!listLoading && !list.length" class="tbl-empty">
              <td colspan="8">暂无数据</td>
            </tr>
            <tr v-for="(row, index) in list" :key="row.id">
              <td class="primary-cell">
                <strong>{{ row.username }}</strong
                ><small>{{ row.email || '未绑定邮箱' }}</small>
              </td>
              <td>
                <span
                  class="chip"
                  :class="row.roleId === 1 ? 'violet' : 'info'"
                  >{{ roleFilter(row.roleId) }}</span
                >
              </td>
              <td class="account-actions-cell">
                <div class="traffic-label">
                  <span class="muted num">{{
                    getFlow(Number(row.upload || 0) + Number(row.download || 0))
                  }}</span
                  ><span class="faint num">{{
                    row.quota < 0 ? '不限' : getFlow(row.quota)
                  }}</span>
                </div>
                <div
                  class="meter"
                  :class="{
                    warn: accountUsagePct(row) > 70,
                    bad: accountUsagePct(row) > 92
                  }"
                >
                  <i :style="{ width: accountUsagePct(row) + '%' }"></i>
                </div>
              </td>
              <td class="muted account-expiry-column">
                <span v-if="row.lastLoginTime === 0">首次登录后计算</span>
                <time v-else class="account-expiry-value">
                  <span>{{ timePart(row.expireTime, 0) }}</span>
                  <span>{{ timePart(row.expireTime, 1) }}</span>
                </time>
              </td>
              <td class="muted">
                {{
                  row.lastLoginTime
                    ? timeStampToDate(row.lastLoginTime, false)
                    : '从未登录'
                }}
              </td>
              <td>
                <span class="chip" :class="row.deleted === 0 ? 'ok' : 'bad'"
                  ><span class="dot"></span
                  >{{ row.deleted === 0 ? '正常' : '停用' }}</span
                >
              </td>
              <td>
                <div class="row-actions">
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="icon-btn"
                    type="button"
                    title="编辑"
                    @click="handleUpdate(row)"
                  >
                    <app-icon name="edit" />
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
                    class="icon-btn"
                    type="button"
                    title="重置流量"
                    @click="handleReset(row)"
                  >
                    <app-icon name="refresh" />
                  </button>
                  <button
                    v-if="
                      checkPermission(['sysadmin', 'admin']) &&
                      row.lastLoginTime !== 0
                    "
                    class="icon-btn"
                    type="button"
                    title="复制订阅"
                    @click="handleClashSubscribeForSb(row)"
                  >
                    <app-icon name="document-copy" />
                  </button>
                  <button
                    v-if="checkPermission(['sysadmin'])"
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
      :title="textMap[dialogStatus]"
      :visible.sync="dialogFormVisible"
    >
      <liquid-form
        ref="dataForm"
        :rules="dialogStatus === 'create' ? createRules : updateRules"
        :model="temp"
        label-position="left"
      >
        <liquid-form-item
          v-if="dialogStatus === 'create'"
          :label="$t('table.username')"
          prop="username"
        >
          <liquid-input
            v-model="temp.username"
            :placeholder="$t('table.username')"
            clearable
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.pass')" prop="pass">
          <liquid-input
            v-model="temp.pass"
            type="password"
            :placeholder="$t('table.pass')"
            clearable
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.editQuota')" prop="quota">
          <liquid-number-input
            v-model.number="temp.quota"
            controls-position="right"
            type="number"
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.email')" prop="email">
          <liquid-input
            v-model="temp.email"
            :placeholder="$t('table.email')"
            clearable
          />
        </liquid-form-item>
        <liquid-form-item :label="$t('table.status')" prop="deleted">
          <liquid-switch
            v-model="temp.deleted"
            :active-text="$t('table.enable')"
            :inactive-text="$t('table.disable')"
            :active-value="0"
            :inactive-value="1"
          >
          </liquid-switch>
        </liquid-form-item>
        <liquid-form-item :label="$t('table.expireTime')" prop="expireTime">
          <liquid-date-picker
            v-model="temp.expireTime"
            type="datetime"
            value-format="timestamp"
            :placeholder="$t('table.expireTime')"
          />
        </liquid-form-item>
      </liquid-form>
      <div slot="footer" class="dialog-footer">
        <liquid-button @click="dialogFormVisible = false"
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
    <import-tip
      ref="importTip"
      :dialog-form-visible.sync="importVisible"
      :label="$t('table.coverByAccountName')"
      :import-data="importData"
      :download-template="downloadTemplate"
    />
    <BatchOperation
      ref="batchOperationForm"
      :dialog-form-visible-props.sync="batchOperationDialogFormVisible"
      :get-list-props="getList"
    />
  </div>
</template>

<script>
import {
  createAccount,
  deleteAccountById,
  exportAccount,
  exportAccountUnused,
  exportSubscribe,
  importAccount,
  resetAccountDownloadAndUpload,
  selectAccountPage,
  updateAccountById
} from '@/api/account'
import Pagination from '@/components/Pagination'
import ImportTip from '@/components/ImportTip'
import latestListRequest from '@/mixins/latest-list-request'

import { Message, MessageBox } from '@/utils/liquid-feedback'
import { timeStampToDate } from '@/utils'
import { byteToMb, getFlow, mbToByte } from '@/utils/account'
import { selectRoleList } from '@/api/role'
import checkPermission from '@/utils/permission'
import { setting } from '@/api/system'
import { downloadTemplate } from '@/api/file-task'
import BatchOperation from '@/views/account/list/compoments/BatchOperation'
import copy from 'copy-to-clipboard'

export default {
  name: 'AccountListPage',
  filters: {
    deletedFilter(deleted) {
      const deletedMap = {
        0: 'success',
        1: 'danger'
      }
      return deletedMap[deleted]
    }
  },
  components: { BatchOperation, Pagination, ImportTip },
  mixins: [latestListRequest],
  data() {
    const validateUsername = (rule, value, callback) => {
      if (this.temp.username.trim().indexOf('admin') >= 0) {
        callback(new Error(this.$t('valid.usernameNotExistAdmin')))
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
      orderFieldArr: ['role_id', 'create_time'],
      listQuery: {
        pageNum: 1,
        pageSize: 20,
        username: undefined,
        deleted: undefined,
        orderFields: 'role_id,create_time',
        orderBy: 'desc',
        lastLoginTime: undefined
      },
      temp: {
        id: undefined,
        quota: 0,
        download: 0,
        upload: 0,
        username: undefined,
        pass: undefined,
        email: undefined,
        roleId: 3,
        deleted: 0,
        lastLoginTime: 0,
        expireTime: new Date().getTime(),
        createTime: new Date()
      },
      dialogFormVisible: false,
      batchOperationDialogFormVisible: false,
      textMap: {
        update: this.$t('table.edit'),
        create: this.$t('table.add')
      },
      importVisible: false,
      deletedList: [
        { value: 0, label: this.$t('table.enable') },
        { value: 1, label: this.$t('table.disable') }
      ],
      lastLoginTimeList: [
        { value: 0, label: this.$t('table.no') },
        { value: 1, label: this.$t('table.yes') }
      ],
      orderFieldList: [
        { value: 'quota', label: this.$t('table.quota') },
        { value: 'role_id', label: this.$t('table.role') },
        { value: 'last_login_time', label: this.$t('table.lastLoginTime') },
        { value: 'expire_time', label: this.$t('table.expireTime') },
        { value: 'deleted', label: this.$t('table.deleted') },
        { value: 'create_time', label: this.$t('table.createTime') }
      ],
      orderByList: [
        { value: 'desc', label: this.$t('table.desc') },
        { value: 'asc', label: this.$t('table.asc') }
      ],
      createRules: {
        username: [
          {
            required: true,
            message: this.$t('valid.username'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.usernameRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.usernameElement'),
            trigger: ['change', 'blur']
          },
          {
            validator: validateUsername,
            trigger: ['change', 'blur']
          }
        ],
        pass: [
          {
            required: true,
            message: this.$t('valid.pass'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.passRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.passElement'),
            trigger: ['change', 'blur']
          }
        ],
        quota: [
          {
            required: true,
            message: this.$t('valid.quota'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: -1,
            max: 1024000,
            message: this.$t('valid.quotaRange'),
            trigger: ['change', 'blur']
          }
        ],
        email: [
          {
            min: 4,
            max: 64,
            message: this.$t('valid.emailRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^([A-Za-z0-9_.-])+@(163.com|126.com|qq.com|gmail.com)$/,
            message: this.$t('valid.emailElement'),
            trigger: ['change', 'blur']
          }
        ],
        deleted: [
          {
            required: true,
            message: this.$t('valid.deleted'),
            trigger: ['change', 'blur']
          }
        ],
        expireTime: [
          {
            required: true,
            message: this.$t('valid.expireTime'),
            trigger: ['change', 'blur']
          }
        ]
      },
      updateRules: {
        username: [
          {
            required: true,
            message: this.$t('valid.username'),
            trigger: ['change', 'blur']
          },
          {
            min: 6,
            max: 20,
            message: this.$t('valid.usernameRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.usernameElement'),
            trigger: ['change', 'blur']
          },
          {
            validator: validateUsername,
            trigger: ['change', 'blur']
          }
        ],
        pass: [
          {
            min: 6,
            max: 20,
            message: this.$t('valid.passRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^[A-Za-z0-9]+$/,
            message: this.$t('valid.passElement'),
            trigger: ['change', 'blur']
          }
        ],
        quota: [
          {
            required: true,
            message: this.$t('valid.quota'),
            trigger: ['change', 'blur']
          },
          {
            type: 'number',
            min: -1,
            max: 1024000,
            message: this.$t('valid.quotaRange'),
            trigger: ['change', 'blur']
          }
        ],
        email: [
          {
            min: 4,
            max: 64,
            message: this.$t('valid.emailRange'),
            trigger: ['change', 'blur']
          },
          {
            pattern: /^([A-Za-z0-9_.-])+@(163.com|126.com|qq.com|gmail.com)$/,
            message: this.$t('valid.emailElement'),
            trigger: ['change', 'blur']
          }
        ],
        deleted: [
          {
            required: true,
            message: this.$t('valid.deleted'),
            trigger: ['change', 'blur']
          }
        ],
        expireTime: [
          {
            required: true,
            message: this.$t('valid.expireTime'),
            trigger: ['change', 'blur']
          }
        ]
      },
      dialogStatus: '',
      roleList: []
    }
  },
  created() {
    this.setRoleList()
    this.getList()
  },
  methods: {
    handleResetAllAccountTraffic() {
      this.$message({
        type: 'info',
        showClose: true,
        message: '全部用户流量统计重置接口待接入，当前未修改任何数据'
      })
    },
    timePart(timestamp, index) {
      return timeStampToDate(timestamp, false).split(' ')[index] || '—'
    },
    accountUsagePct(row) {
      if (row.quota < 0) return 0
      if (!row.quota) return 100
      return Math.max(
        0,
        Math.min(
          100,
          Math.round(
            ((Number(row.upload || 0) + Number(row.download || 0)) /
              Number(row.quota)) *
              100
          )
        )
      )
    },
    checkPermission,
    getFlow,
    timeStampToDate,
    setRoleList() {
      selectRoleList().then((response) => {
        const { data } = response
        this.roleList = data
      })
    },
    getList() {
      const request = this.beginListRequest()
      this.listQuery.orderFields = this.orderFieldArr.join(',')
      return selectAccountPage(this.listQuery).then((response) => {
        if (!this.ownsListRequest(request)) return
        this.list = response.data.accounts
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
        quota: 0,
        download: 0,
        upload: 0,
        username: undefined,
        pass: undefined,
        email: undefined,
        roleId: 3,
        deleted: 0,
        expireTime: new Date().getTime(),
        createTime: new Date()
      }
    },
    handleFilter() {
      this.listQuery.pageNum = 1
      this.getList()
    },
    handleCreate() {
      this.resetTemp()
      setting().then((response) => {
        if (response.data.registerEnable === 1) {
          this.temp.quota = response.data.registerQuota
          this.temp.expireTime =
            new Date().getTime() + response.data.registerExpireDays * 86400000
        }
      })
      this.dialogStatus = 'create'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleUpdate(row) {
      this.temp = Object.assign({}, row)
      this.temp.quota = byteToMb(row.quota)
      this.dialogStatus = 'update'
      this.dialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['dataForm'].clearValidate()
      })
    },
    handleDelete(row, index) {
      MessageBox.confirm(
        this.$t('confirm.deleteUser'),
        this.$t('confirm.warn'),
        {
          confirmButtonText: this.$t('confirm.yes'),
          cancelButtonText: this.$t('confirm.cancel'),
          type: 'warning'
        }
      ).then(() => {
        const tempData = Object.assign({}, row)
        deleteAccountById(tempData).then(() => {
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
          createAccount(this.temp).then(() => {
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
    },
    updateData() {
      this.$refs['dataForm'].validate((valid) => {
        if (valid) {
          const tempData = Object.assign({}, this.temp)
          updateAccountById(tempData).then(() => {
            const index = this.list.findIndex((v) => v.id === this.temp.id)
            this.temp.quota = mbToByte(this.temp.quota)
            this.list.splice(index, 1, this.temp)
            this.dialogFormVisible = false
            this.$notify({
              title: 'Success',
              message: this.$t('confirm.modifySuccess'),
              type: 'success',
              duration: 2000
            })
          })
        }
      })
    },
    roleFilter(roleId) {
      let role = this.roleList.find((item) => item.id === roleId)
      if (role && role.desc) {
        return role.desc
      } else {
        return ''
      }
    },
    handleReset(row) {
      MessageBox.confirm(
        this.$t('confirm.handleReset'),
        this.$t('confirm.warn'),
        {
          confirmButtonText: this.$t('confirm.yes'),
          cancelButtonText: this.$t('confirm.cancel'),
          type: 'warning'
        }
      ).then(() => {
        const tempData = Object.assign({}, row)
        resetAccountDownloadAndUpload(tempData).then(() => {
          this.getList()
          this.$notify({
            title: 'Success',
            message: this.$t('confirm.resetSuccess'),
            type: 'success',
            duration: 2000
          })
        })
      })
    },
    async importData(params) {
      const valid = await this.$refs.importTip.$refs.dataForm.validate()
      if (!valid) return false
      const formData = new FormData()
      formData.append('file', params.file)
      formData.append('cover', this.$refs.importTip.temp.cover)
      await importAccount(formData)
      this.importVisible = false
      this.$notify({
        title: 'Success',
        message: this.$t('confirm.taskSubmitSuccess'),
        type: 'success',
        duration: 2000
      })
      return true
    },
    handleImport() {
      this.importVisible = true
    },
    handleExport() {
      exportAccount().then(() => {
        this.importVisible = false
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.taskSubmitSuccess').toString(),
          type: 'success',
          duration: 2000
        })
      })
    },
    handleCreateBatch() {
      this.batchOperationDialogFormVisible = true
      this.$nextTick(() => {
        this.$refs['batchOperationForm'].$refs['dataForm'].clearValidate()
      })
    },
    exportAccountUnused() {
      exportAccountUnused().then(() => {
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.taskSubmitSuccess').toString(),
          type: 'success',
          duration: 2000
        })
      })
    },
    downloadTemplate() {
      downloadTemplate({ id: 1 }).then((res) => {
        // 将二进制文件转化为可访问的url
        const blob = new Blob([res.data], {
          type: 'application/octet-stream'
        })
        let url = window.URL.createObjectURL(blob)
        let a = document.createElement('a')
        document.body.appendChild(a)
        a.href = url
        let dis = res.headers['content-disposition']
        a.download = dis.split('attachment; filename=')[1]
        // 模拟点击下载
        a.click()
        window.URL.revokeObjectURL(url)
        this.$notify({
          title: 'Success',
          message: this.$t('confirm.taskDownloadSuccess'),
          type: 'success',
          duration: 2000
        })
      })
    },
    handleClashSubscribeForSb(row) {
      exportSubscribe({
        id: row.id,
        client: 'sing-box',
        template: 'tun'
      }).then((response) => {
        if (copy(new URL(response.data, window.location.origin).toString())) {
          Message({
            showClose: true,
            message: this.$t('confirm.urlCopySuccess').toString(),
            type: 'success'
          })
        } else {
          Message({
            showClose: true,
            message: this.$t('confirm.urlCopyFail').toString(),
            type: 'error'
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
