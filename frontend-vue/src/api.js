const API_BASE_URL = import.meta.env?.VITE_API_BASE_URL || ''
const AUTH_TOKEN_KEY = 'player-stats-admin-token'

export function getAuthToken() {
  return localStorage.getItem(AUTH_TOKEN_KEY) || ''
}

export function setAuthToken(token) {
  localStorage.setItem(AUTH_TOKEN_KEY, token)
}

export function clearAuthToken() {
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

export async function apiGet(path, params = {}, options = {}) {
  const url = new URL(`${API_BASE_URL}${path}`, window.location.origin)
  Object.entries(params)
    .filter(([, value]) => value !== null && value !== undefined && value !== '')
    .forEach(([key, value]) => url.searchParams.set(key, value))

  const response = await fetch(url, {
    headers: authHeaders({}, options),
    signal: options.signal
  })
  return parseResponse(response)
}

export async function apiPost(path, params = {}, options = {}) {
  const url = new URL(`${API_BASE_URL}${path}`, window.location.origin)
  Object.entries(params)
    .filter(([, value]) => value !== null && value !== undefined && value !== '')
    .forEach(([key, value]) => url.searchParams.set(key, value))

  const response = await fetch(url, {
    method: 'POST',
    headers: authHeaders({
      Accept: 'application/json'
    }, options),
    signal: options.signal
  })
  return parseResponse(response)
}

export async function apiPostJson(path, body = {}, options = {}) {
  const url = new URL(`${API_BASE_URL}${path}`, window.location.origin)
  const response = await fetch(url, {
    method: 'POST',
    headers: authHeaders({
      Accept: 'application/json',
      'Content-Type': 'application/json'
    }, options),
    body: JSON.stringify(body),
    signal: options.signal
  })
  return parseResponse(response)
}

export async function apiPutJson(path, body = {}, options = {}) {
  const url = new URL(`${API_BASE_URL}${path}`, window.location.origin)
  const response = await fetch(url, {
    method: 'PUT',
    headers: authHeaders({
      Accept: 'application/json',
      'Content-Type': 'application/json'
    }, options),
    body: JSON.stringify(body),
    signal: options.signal
  })
  return parseResponse(response)
}

export async function apiDelete(path, params = {}, options = {}) {
  const url = new URL(`${API_BASE_URL}${path}`, window.location.origin)
  Object.entries(params)
    .filter(([, value]) => value !== null && value !== undefined && value !== '')
    .forEach(([key, value]) => url.searchParams.set(key, value))

  const response = await fetch(url, {
    method: 'DELETE',
    headers: authHeaders({
      Accept: 'application/json'
    }, options)
  })
  return parseResponse(response)
}

function authHeaders(headers = {}, options = {}) {
  if (options.auth === false) {
    return headers
  }
  const token = getAuthToken()
  if (!token) {
    return headers
  }
  return {
    ...headers,
    Authorization: `Bearer ${token}`
  }
}

async function parseResponse(response) {
  let payload
  try { payload = await response.json() }
  catch (err) {
    if (err.name === 'AbortError') throw err
    if (response.ok && response.status !== 204) throw new Error('服务器响应格式无效，请重试')
    payload = {}
  }
  if (!response.ok) {
    if (response.status === 401) {
      clearAuthToken()
    }
    const error = new Error(payload.message || `请求失败: ${response.status}`)
    error.status = response.status
    throw error
  }
  return payload
}
