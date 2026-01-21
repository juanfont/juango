// User & Session Types
export interface User {
  id: string
  email: string
  name: string
  is_admin: boolean
  display_name: string
  profile_pic_url: string
  last_login?: string
  created_at: string
  modified_at: string
}

export interface SessionResponse {
  authenticated: boolean
  user?: User
  reason?: string
  impersonation?: ImpersonationState
}

// Admin Mode Types
export interface AdminModeState {
  enabled: boolean
  since: string
  reason: string
  ip_address: string
}

export interface AdminModeStatusResponse {
  is_admin: boolean
  admin_mode?: AdminModeState
}

export interface AdminModeEnableRequest {
  reason: string
}

export interface AdminModeEnableResponse {
  message: string
  state?: AdminModeState
}

export interface AdminModeDisableResponse {
  message: string
}

// Impersonation Types
export interface ImpersonationState {
  enabled: boolean
  since: string
  reason: string
  target_user_id: string
  target_user_email: string
  target_user_name: string
  original_admin_id: string
  ip_address: string
}

export interface ImpersonationStartRequest {
  target_user_id: string
  reason: string
}

export interface ImpersonationStartResponse {
  message: string
  impersonation: ImpersonationState
}

export interface ImpersonationStopResponse {
  message: string
}

export interface ImpersonationStatusResponse {
  active: boolean
  impersonation?: ImpersonationState
}

// Audit Logs
export interface AuditLog {
  id: number
  timestamp: string
  actor_user_id?: string
  actor_name: string
  action: string
  resource_type: string
  resource_id: string
  changes?: Record<string, unknown>
  ip_address?: string
  user_agent?: string
}

export interface AuditLogFilters {
  actor_user_id?: string
  action?: string
  resource_type?: string
  resource_id?: string
  start_time?: string
  end_time?: string
  limit?: number
  offset?: number
}

// Notifications
export interface Notification {
  id: string
  user_id: string
  type: string
  priority: "urgent" | "warning" | "info" | "success"
  title: string
  message: string
  link?: string
  metadata?: Record<string, unknown>
  read_at?: string
  created_at: string
}
