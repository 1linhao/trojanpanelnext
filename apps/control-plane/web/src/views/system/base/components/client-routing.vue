<template>
  <div class="routing-settings">
    <p role="status">{{ active ? '活动版本 #' + active.id + ' · ' + active.name : '当前使用各客户端原有的订阅模板' }}</p>
    <p>发布后，账号下次刷新 Clash.Meta 或 sing-box TUN 订阅时生效。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <button v-if="!ready && !busy" class="cap" @click="initialize">重新加载</button>
    <fieldset v-if="ready" :disabled="busy" class="editor">
      <section v-if="releases.length">
        <h3>版本历史</h3>
        <div class="fields">
          <div class="select-field release-version">
            <label for="routing-release">版本</label>
            <liquid-select id="routing-release" v-model="selected">
              <option v-for="r in releases" :key="r.id" :value="r.id">
                #{{ r.id }} · {{ r.name }} · {{ new Date(r.createdAt).toLocaleString() }}{{ r.active ? '（活动）' : '' }}
              </option>
            </liquid-select>
          </div>
          <button class="cap small" @click="loadCopy">查看 / 复制到编辑器</button>
          <button class="cap small" :disabled="selected === activeId" @click="activate(selected)">恢复此版本</button>
          <button class="cap small" :disabled="selected === activeId" @click="remove">删除此版本</button>
          <button v-if="active" class="cap small" @click="activate(0)">停用分流</button>
        </div>
      </section>
      <section>
        <h3>基础模板</h3>
        <div class="fields">
          <label>版本名称<input v-model.trim="draft.name" maxlength="64" placeholder="例如：办公网络"></label>
          <label>Mihomo 混合监听端口<input v-model.number="draft.base.mixedPort" type="number" min="1" max="65535"></label>
          <label>sing-box TUN MTU<input v-model.number="draft.base.tunMtu" type="number" min="1280" max="9000"></label>
          <div class="select-field">
            <label for="routing-log-level">日志级别</label>
            <liquid-select id="routing-log-level" v-model="draft.base.logLevel">
              <option>debug</option><option>info</option><option>error</option>
            </liquid-select>
          </div>
        </div>
      </section>
      <section>
        <h3>节点组</h3>
        <p>每组可选同一服务器上的多个节点。客户端只会收到账号有权使用且兼容的成员。</p>
        <div v-for="(g, i) in draft.traffic.groups" :key="g.id" class="routing-row">
          <div class="fields">
            <label>组名<input v-model.trim="g.name" maxlength="64"></label>
            <div class="select-field">
              <label :for="'routing-server-' + i">服务器</label>
              <liquid-select :id="'routing-server-' + i" :value="g.serverId" @input="setGroupServer(g, $event)">
                <option :value="0" disabled>选择服务器</option>
                <option v-if="g.serverId && !servers.some(s => s.id === g.serverId)" :value="g.serverId">已移除 #{{ g.serverId }}</option>
                <option v-for="s in servers" :key="s.id" :value="s.id">{{ s.name }} (#{{ s.id }})</option>
              </liquid-select>
            </div>
            <label class="check"><input v-model="g.autoTest" type="checkbox">自动测速选路</label>
            <button class="cap small" :disabled="inUse(g.id)" :title="inUse(g.id) ? '先更改引用此组的规则与默认策略' : ''" @click="draft.traffic.groups.splice(i, 1)">移除组</button>
          </div>
          <div v-if="g.serverId" class="members" role="group" :aria-label="g.name + '成员'">
            <label v-for="n in memberOptions(g)" :key="n.id" class="check">
              <input v-model="g.nodeIds" type="checkbox" :value="n.id">{{ n.name }} (#{{ n.id }})
            </label>
          </div>
        </div>
        <button class="cap small" :disabled="!servers.length" @click="addGroup">添加节点组</button>
        <p v-if="!servers.length">有可用节点的服务器会出现在这里。</p>
      </section>
      <section>
        <h3>流量规则</h3>
        <p>按从上到下的顺序匹配。首条命中的规则决定出口。</p>
        <div v-for="(r, i) in draft.traffic.rules" :key="i" class="routing-row fields">
          <div class="select-field">
            <label :for="'routing-rule-kind-' + i">第 {{ i + 1 }} 条</label>
            <liquid-select :id="'routing-rule-kind-' + i" v-model="r.kind">
              <option value="domain">精确域名</option><option value="suffix">域名后缀</option><option value="ip">IP / CIDR</option>
            </liquid-select>
          </div>
          <label>目标<input v-model.trim="r.value" :placeholder="r.kind === 'ip' ? '192.0.2.0/24' : 'example.com'"></label>
          <div class="select-field">
            <label :for="'routing-rule-target-' + i">出口</label>
            <liquid-select :id="'routing-rule-target-' + i" v-model="r.target">
              <option v-for="t in targets" :key="t.id" :value="t.id">{{ t.name }}</option>
            </liquid-select>
          </div>
          <button class="cap small" :disabled="!i" @click="move(i, -1)">上移</button>
          <button class="cap small" :disabled="i + 1 === draft.traffic.rules.length" @click="move(i, 1)">下移</button>
          <button class="cap small" @click="draft.traffic.rules.splice(i, 1)">移除</button>
        </div>
        <button class="cap small" @click="addRule">添加流量规则</button>
        <div class="fields defaults">
          <div class="select-field">
            <label for="routing-default-target">未匹配流量</label>
            <liquid-select id="routing-default-target" v-model="draft.traffic.default">
              <option v-for="t in targets" :key="t.id" :value="t.id">{{ t.name }}</option>
            </liquid-select>
          </div>
          <div class="select-field">
            <label for="routing-empty-target">所选出口无可用节点</label>
            <liquid-select id="routing-empty-target" v-model="draft.traffic.empty">
              <option value="REJECT">拒绝连接</option><option value="DIRECT">直连</option>
            </liquid-select>
          </div>
        </div>
      </section>
      <section>
        <h3>DNS 规则</h3>
        <p>填写 UDP 解析器的 IP 地址。域名匹配多个后缀时，使用最长后缀。</p>
        <label>默认 DNS<input v-model.trim="draft.dns.default" placeholder="1.1.1.1"></label>
        <div v-for="(r, i) in draft.dns.rules" :key="i" class="routing-row fields">
          <label>域名后缀<input v-model.trim="r.suffix" placeholder="example.com"></label>
          <label>解析器 IP<input v-model.trim="r.server"></label>
          <button class="cap small" @click="draft.dns.rules.splice(i, 1)">移除</button>
        </div>
        <button class="cap small" @click="draft.dns.rules.push({ suffix: '', server: '' })">添加 DNS 规则</button>
      </section>
      <button class="cap primary" @click="publish">校验并发布新版本</button>
    </fieldset>
  </div>
</template>

<script>
import { routingSettings, publishRouting, activateRouting, deleteRouting } from '@/api/client-routing'
import { Message, MessageBox } from '@/utils/liquid-feedback'

function newProfile() {
  return {
    name: '',
    base: { mixedPort: 7890, tunMtu: 1500, logLevel: 'info' },
    traffic: { groups: [], rules: [], default: 'PROXY', empty: 'REJECT' },
    dns: { default: '1.1.1.1', rules: [] }
  }
}

export default {
  name: 'ClientRoutingSettings',
  data() {
    return { nodes: [], releases: [], draft: newProfile(), selected: 0, ready: false, busy: false, error: '' }
  },
  computed: {
    active() { return this.releases.find(r => r.active) },
    activeId() { return this.active ? this.active.id : 0 },
    servers() {
      const items = new Map(this.nodes.map(n => [n.serverId, { id: n.serverId, name: n.serverName }]))
      return Array.from(items.values())
    },
    targets() {
      return [
        ...this.draft.traffic.groups.map(g => ({ id: g.id, name: g.name || '未命名组' })),
        { id: 'PROXY', name: '全部可用节点（手选）' }, { id: 'DIRECT', name: '直连' }, { id: 'REJECT', name: '拒绝连接' }
      ]
    }
  },
  created() { this.initialize() },
  methods: {
    async run(action) {
      if (this.busy) return
      this.busy = true
      this.error = ''
      try { await action() } catch (e) { if (e !== 'cancel') this.error = e.message || '操作失败' }
      finally { this.busy = false }
    },
    async refresh(selectActive = true) {
      const { data } = await routingSettings()
      this.nodes = data.nodes
      this.releases = data.releases
      if (selectActive) this.selected = this.activeId || this.releases[0]?.id || 0
    },
    async read(id) {
      const { data } = await routingSettings(id)
      data.traffic.groups = data.traffic.groups || []
      data.traffic.rules = data.traffic.rules || []
      data.dns.rules = data.dns.rules || []
      this.draft = data
    },
    initialize() {
      return this.run(async () => {
        await this.refresh()
        if (this.selected) await this.read(this.selected)
        this.ready = true
      })
    },
    loadCopy() {
      return this.run(async () => {
        await this.refresh(false)
        await this.read(this.selected)
      })
    },
    memberOptions(group) {
      const available = this.nodes.filter(n => n.serverId === group.serverId)
      const found = new Set(available.map(n => n.id))
      const missing = group.nodeIds.filter(id => !found.has(id)).map(id => ({ id, name: '已移除或迁移' }))
      return available.concat(missing)
    },
    inUse(id) {
      return this.draft.traffic.default === id || this.draft.traffic.rules.some(r => r.target === id)
    },
    addGroup() {
      let n = this.draft.traffic.groups.length + 1
      while (this.draft.traffic.groups.some(g => g.id === 'g' + n)) n++
      this.draft.traffic.groups.push({ id: 'g' + n, name: '', serverId: 0, nodeIds: [], autoTest: true })
    },
    setGroupServer(group, serverId) {
      if (group.serverId === serverId) return
      group.serverId = serverId
      group.nodeIds = []
    },
    addRule() {
      this.draft.traffic.rules.push({ kind: 'suffix', value: '', target: this.targets[0].id })
    },
    move(i, shift) {
      const rules = this.draft.traffic.rules
      const [rule] = rules.splice(i, 1)
      rules.splice(i + shift, 0, rule)
    },
    publish() {
      return this.run(async () => {
        try { await publishRouting(this.draft) }
        finally { await this.refresh(false) }
        this.selected = this.activeId
        Message.success('分流版本已发布')
      })
    },
    activate(id) {
      return this.run(async () => {
        try { await activateRouting(id) }
        finally { await this.refresh(false) }
        this.selected = this.activeId || this.releases[0]?.id || 0
        if (id) await this.read(id)
        Message.success(id ? '已恢复分流版本' : '已恢复各客户端原有模板')
      })
    },
    remove() {
      return this.run(async () => {
        await MessageBox.confirm('确定删除历史版本 #' + this.selected + '？', '删除版本')
        await deleteRouting(this.selected)
        await this.refresh()
        Message.success('历史版本已删除')
      })
    }
  }
}
</script>

<style scoped>
.routing-settings { color: var(--ink); padding-block: 12px; }
.routing-settings p { color: var(--ink-2); margin: 6px 0 12px; }
.routing-settings [role="alert"] { color: var(--ink); font-weight: 600; }
.editor { min-width: 0; border: 0; padding: 0; margin: 0; }
.editor section { padding-block: 18px; border-top: 1px solid var(--rim); }
.editor h3 { font-size: 1.05rem; margin: 0 0 12px; }
.fields, .members { display: flex; align-items: end; gap: 10px; flex-wrap: wrap; }
.editor label, .select-field { display: grid; gap: 5px; font-size: 14px; max-width: 100%; }
.select-field { grid-template-columns: minmax(0, 1fr); width: 12em; }
.release-version { width: var(--control-max-width); }
.editor label.check { display: inline-flex; align-items: center; gap: 6px; padding: 6px 0; }
.editor input:not([type="checkbox"]) { max-width: 100%; box-sizing: border-box; min-height: 36px; padding: 5px 9px; border: 1px solid var(--rim); border-radius: 8px; color: var(--ink); background: var(--glass-popover); font: inherit; }
.editor input[type="number"] { width: 10em; }
.editor .routing-row { padding: 12px; margin-bottom: 8px; border: 1px solid var(--rim); border-radius: 10px; }
.members, .defaults { margin-top: 10px; }
.editor button:disabled { opacity: .55; }
</style>
