import assert from 'node:assert/strict'
import { createServer as createHttpServer } from 'node:http'
import { test } from 'node:test'
import { createServer } from 'vite'

test('Vite preserves API paths, query, method, body, auth and backend status', async () => {
  const backend = createHttpServer(async (request, response) => {
    const chunks = []
    for await (const chunk of request) chunks.push(chunk)
    response.writeHead(422, { 'Content-Type': 'application/json' })
    response.end(JSON.stringify({
      url: request.url,
      method: request.method,
      authorization: request.headers.authorization,
      body: Buffer.concat(chunks).toString()
    }))
  })
  await new Promise((resolve) => backend.listen(0, '127.0.0.1', resolve))
  const oldTarget = process.env.MOCK_API_TARGET
  process.env.MOCK_API_TARGET = `http://127.0.0.1:${backend.address().port}`
  let frontend
  try {
    frontend = await createServer({ server: { port: 0 }, logLevel: 'error' })
    await frontend.listen()
    const apiPath = '/api/probe?prefix=%2Fapi&date=2026-08'
    const payload = JSON.stringify({ name: 'proxy fixture' })
    const response = await fetch(`http://127.0.0.1:${frontend.httpServer.address().port}${apiPath}`, {
      method: 'POST',
      headers: { Authorization: 'Bearer proxy-fixture', 'Content-Type': 'application/json' },
      body: payload
    })
    assert.equal(response.status, 422)
    assert.deepEqual(await response.json(), {
      url: apiPath,
      method: 'POST',
      authorization: 'Bearer proxy-fixture',
      body: payload
    })
  } finally {
    await frontend?.close()
    await new Promise((resolve) => backend.close(resolve))
    if (oldTarget === undefined) delete process.env.MOCK_API_TARGET
    else process.env.MOCK_API_TARGET = oldTarget
  }
})
