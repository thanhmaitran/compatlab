import { post, setToken } from './api-client'

interface TokenResponse {
  token: string
}

export async function login(email: string, password: string): Promise<void> {
  const res = await post<TokenResponse>('/auth/login', { email, password })
  setToken(res.token)
}

export async function register(organization: string, email: string, password: string): Promise<void> {
  const res = await post<TokenResponse>('/auth/register', { organization, email, password })
  setToken(res.token)
}

export function logout(): void {
  setToken(null)
}
