import { useState, useEffect } from "react"
import { Bell } from "lucide-react"
import { Button } from "./ui/button"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "./ui/popover"
import { ScrollArea } from "./ui/scroll-area"
import { Badge } from "./ui/badge"
import { getApiClient } from "../lib/api"
import type { Notification } from "../lib/types"
import { cn } from "../lib/utils"

interface NotificationBellProps {
  className?: string
  pollInterval?: number // in milliseconds
}

export function NotificationBell({ className, pollInterval = 30000 }: NotificationBellProps) {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [unreadCount, setUnreadCount] = useState(0)
  const [isOpen, setIsOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)

  const apiClient = getApiClient()

  const fetchNotifications = async () => {
    try {
      const data = await apiClient.getNotifications({ limit: 10 })
      setNotifications(data)
      const count = await apiClient.getUnreadNotificationCount()
      setUnreadCount(count.count)
    } catch (error) {
      console.error("Failed to fetch notifications:", error)
    }
  }

  const fetchUnreadCount = async () => {
    try {
      const count = await apiClient.getUnreadNotificationCount()
      setUnreadCount(count.count)
    } catch (error) {
      console.error("Failed to fetch unread count:", error)
    }
  }

  useEffect(() => {
    fetchUnreadCount()
    const interval = setInterval(fetchUnreadCount, pollInterval)
    return () => clearInterval(interval)
  }, [pollInterval])

  useEffect(() => {
    if (isOpen) {
      setIsLoading(true)
      fetchNotifications().finally(() => setIsLoading(false))
    }
  }, [isOpen])

  const handleMarkAsRead = async (id: string) => {
    try {
      await apiClient.markNotificationAsRead(id)
      setNotifications(prev =>
        prev.map(n => n.id === id ? { ...n, read_at: new Date().toISOString() } : n)
      )
      setUnreadCount(prev => Math.max(0, prev - 1))
    } catch (error) {
      console.error("Failed to mark notification as read:", error)
    }
  }

  const handleMarkAllAsRead = async () => {
    try {
      await apiClient.markAllNotificationsAsRead()
      setNotifications(prev =>
        prev.map(n => ({ ...n, read_at: new Date().toISOString() }))
      )
      setUnreadCount(0)
    } catch (error) {
      console.error("Failed to mark all notifications as read:", error)
    }
  }

  const getPriorityColor = (priority: Notification["priority"]) => {
    switch (priority) {
      case "urgent":
        return "bg-red-500"
      case "warning":
        return "bg-yellow-500"
      case "success":
        return "bg-green-500"
      default:
        return "bg-blue-500"
    }
  }

  return (
    <Popover open={isOpen} onOpenChange={setIsOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className={cn("relative", className)}>
          <Bell className="h-5 w-5" />
          {unreadCount > 0 && (
            <Badge
              variant="destructive"
              className="absolute -top-1 -right-1 h-5 min-w-5 flex items-center justify-center p-0 text-xs"
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </Badge>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80 p-0" align="end">
        <div className="flex items-center justify-between px-4 py-3 border-b">
          <h4 className="font-semibold">Notifications</h4>
          {unreadCount > 0 && (
            <Button variant="ghost" size="sm" onClick={handleMarkAllAsRead}>
              Mark all as read
            </Button>
          )}
        </div>
        <ScrollArea className="h-80">
          {isLoading ? (
            <div className="flex items-center justify-center h-20">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
            </div>
          ) : notifications.length === 0 ? (
            <div className="flex items-center justify-center h-20 text-muted-foreground">
              No notifications
            </div>
          ) : (
            <div className="divide-y">
              {notifications.map((notification) => (
                <div
                  key={notification.id}
                  className={cn(
                    "p-4 hover:bg-muted/50 cursor-pointer transition-colors",
                    !notification.read_at && "bg-muted/30"
                  )}
                  onClick={() => !notification.read_at && handleMarkAsRead(notification.id)}
                >
                  <div className="flex items-start gap-3">
                    <div className={cn("w-2 h-2 rounded-full mt-2 flex-shrink-0", getPriorityColor(notification.priority))} />
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-sm truncate">{notification.title}</p>
                      <p className="text-sm text-muted-foreground line-clamp-2">{notification.message}</p>
                      <p className="text-xs text-muted-foreground mt-1">
                        {new Date(notification.created_at).toLocaleString()}
                      </p>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </ScrollArea>
      </PopoverContent>
    </Popover>
  )
}
