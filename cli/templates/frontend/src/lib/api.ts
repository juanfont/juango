import type {
  AdminModeDisableResponse,
  AdminModeEnableRequest,
  AdminModeEnableResponse,
  AdminModeStatusResponse,
  ImpersonationStartRequest,
  ImpersonationStartResponse,
  ImpersonationStopResponse,
  ImpersonationStatusResponse,
  SessionResponse,
  Notification,
} from "./types"

const DEFAULT_API_BASE = "/api"

export class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string = DEFAULT_API_BASE) {
    this.baseUrl = baseUrl
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`

    const config: RequestInit = {
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
      ...options,
    }

    const response = await fetch(url, config)

    if (!response.ok) {
      throw new Error(
        `HTTP error! status: ${response.status} ${response.statusText}`
      )
    }

    return await response.json()
  }

  // Auth endpoints
  async checkSession(): Promise<SessionResponse> {
    return this.request<SessionResponse>("/auth/session")
  }

  login(): void {
    window.location.href = `${this.baseUrl}/auth/login`
  }

  async logout(): Promise<void> {
    await this.request("/auth/logout", {
      method: "POST",
    })
    window.location.href = "/login"
  }

  // Admin Mode endpoints
  async getAdminModeStatus(): Promise<AdminModeStatusResponse> {
    return this.request<AdminModeStatusResponse>("/admin/mode/status")
  }

  async enableAdminMode(reason: string): Promise<AdminModeEnableResponse> {
    return this.request<AdminModeEnableResponse>("/admin/mode/enable", {
      method: "POST",
      body: JSON.stringify({ reason } as AdminModeEnableRequest),
    })
  }

  async disableAdminMode(): Promise<AdminModeDisableResponse> {
    return this.request<AdminModeDisableResponse>("/admin/mode/disable", {
      method: "POST",
    })
  }

  // Impersonation endpoints
  async startImpersonation(
    targetUserId: string,
    reason: string
  ): Promise<ImpersonationStartResponse> {
    return this.request<ImpersonationStartResponse>(
      "/admin/impersonate/start",
      {
        method: "POST",
        body: JSON.stringify({
          target_user_id: targetUserId,
          reason,
        } as ImpersonationStartRequest),
      }
    )
  }

  async stopImpersonation(): Promise<ImpersonationStopResponse> {
    return this.request<ImpersonationStopResponse>("/admin/impersonate/stop", {
      method: "POST",
    })
  }

  async getImpersonationStatus(): Promise<ImpersonationStatusResponse> {
    return this.request<ImpersonationStatusResponse>(
      "/admin/impersonate/status"
    )
  }

  // Notifications
  async getNotifications(params?: {
    unread?: boolean
    type?: string
    limit?: number
    offset?: number
  }): Promise<Notification[]> {
    const searchParams = new URLSearchParams()
    if (params?.unread) searchParams.append("unread", "true")
    if (params?.type) searchParams.append("type", params.type)
    if (params?.limit) searchParams.append("limit", params.limit.toString())
    if (params?.offset) searchParams.append("offset", params.offset.toString())

    const query = searchParams.toString()
    return this.request<Notification[]>(
      `/notifications${query ? `?${query}` : ""}`
    )
  }

  async getUnreadNotificationCount(): Promise<{ count: number }> {
    return this.request<{ count: number }>("/notifications/unread/count")
  }

  async markNotificationAsRead(id: string): Promise<{ success: boolean }> {
    return this.request<{ success: boolean }>(`/notifications/${id}/read`, {
      method: "POST",
    })
  }

  async markAllNotificationsAsRead(): Promise<{ success: boolean }> {
    return this.request<{ success: boolean }>("/notifications/read-all", {
      method: "POST",
    })
  }

  async deleteNotification(id: string): Promise<{ success: boolean }> {
    return this.request<{ success: boolean }>(`/notifications/${id}`, {
      method: "DELETE",
    })
  }
}

// Default singleton instance
let defaultClient: ApiClient | null = null

export function createApiClient(baseUrl?: string): ApiClient {
  return new ApiClient(baseUrl)
}

export function getApiClient(): ApiClient {
  if (!defaultClient) {
    defaultClient = new ApiClient()
  }
  return defaultClient
}
