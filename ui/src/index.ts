// Styles
import './styles.css'

// Utilities
export { cn } from './lib/utils'
export { ApiClient, createApiClient } from './lib/api'

// Types
export type {
  User,
  SessionResponse,
  AdminModeState,
  AdminModeStatusResponse,
  AdminModeEnableRequest,
  AdminModeEnableResponse,
  AdminModeDisableResponse,
  ImpersonationState,
  ImpersonationStartRequest,
  ImpersonationStartResponse,
  ImpersonationStopResponse,
  ImpersonationStatusResponse,
  AuditLog,
  AuditLogFilters,
  Notification,
} from './lib/types'

// Contexts
export { AuthProvider, useAuth } from './contexts/AuthContext'
export { AdminProvider, useAdmin } from './contexts/AdminContext'
export { BreadcrumbProvider, useBreadcrumb } from './contexts/BreadcrumbContext'

// Components
export { ProtectedRoute } from './components/ProtectedRoute'
export { AdminGuard } from './components/AdminGuard'
export { AdminModeToggle } from './components/AdminModeToggle'
export { ImpersonationBanner } from './components/ImpersonationBanner'
export { NotificationBell } from './components/NotificationBell'

// UI Components (shadcn)
export { Button, buttonVariants } from './components/ui/button'
export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent } from './components/ui/card'
export {
  Dialog,
  DialogPortal,
  DialogOverlay,
  DialogClose,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from './components/ui/dialog'
export { Label } from './components/ui/label'
export { Textarea } from './components/ui/textarea'
export { Input } from './components/ui/input'
export {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
  TooltipProvider,
} from './components/ui/tooltip'
export { Avatar, AvatarImage, AvatarFallback } from './components/ui/avatar'
export { Separator } from './components/ui/separator'
export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuLabel,
  DropdownMenuGroup,
} from './components/ui/dropdown-menu'
export { ScrollArea, ScrollBar } from './components/ui/scroll-area'
export { Badge, badgeVariants } from './components/ui/badge'
export { Toaster } from './components/ui/sonner'
