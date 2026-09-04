export function getFlow(flowValueBytes) {
  let flow = ''
  //如果流量小于1MB.则显示为KB
  if (flowValueBytes / 1024 < 1024) {
    flow =
      (Math.round(flowValueBytes / 1024) > 0
        ? Math.round(flowValueBytes / 1024)
        : 0) + 'KB'
  } else if (
    flowValueBytes / 1024 >= 1024 &&
    flowValueBytes / 1024 / 1024 < 1024
  ) {
    //如果流量大于1MB且小于1GB的则显示为MB
    flow =
      (Math.round(flowValueBytes / 1024 / 1024) > 0
        ? Math.round(flowValueBytes / 1024 / 1024)
        : 0) + 'MB'
  } else if (flowValueBytes / 1024 / 1024 >= 1024) {
    //如果流量大于1Gb
    let gb_Flow = flowValueBytes / 1024 / 1024 / 1024
    //toFixed(1);四舍五入保留一位小数
    flow = gb_Flow.toFixed(1) + 'GB'
  } else {
    flow = '0KB'
  }
  return flow
}

// byte转mb
export function byteToMb(flowValueBytes) {
  if (flowValueBytes > 0) {
    return flowValueBytes / 1024 / 1024
  }
  return flowValueBytes
}

// mb转byte
export function mbToByte(flowValueBytes) {
  if (flowValueBytes > 0) {
    return flowValueBytes * 1024 * 1024
  }
  return flowValueBytes
}
