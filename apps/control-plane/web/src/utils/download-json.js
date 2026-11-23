import request from '@/utils/request'

export default async function downloadJSON(url, data) {
  const response = await request({
    url,
    method: 'post',
    data,
    responseType: 'blob',
    jsonAttachment: true,
    timeout: 0
  })
  const filename = response.headers['content-disposition'].slice('attachment; filename='.length)
  const objectURL = window.URL.createObjectURL(response.data)
  const link = document.createElement('a')
  link.href = objectURL
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  setTimeout(() => window.URL.revokeObjectURL(objectURL), 0)
}
