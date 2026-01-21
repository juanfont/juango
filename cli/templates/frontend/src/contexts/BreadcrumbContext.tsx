import { createContext, useContext, useState, type ReactNode } from 'react'

export interface BreadcrumbItem {
  label: string
  href?: string
}

interface BreadcrumbContextType {
  items: BreadcrumbItem[]
  setItems: (items: BreadcrumbItem[]) => void
  addItem: (item: BreadcrumbItem) => void
  clearItems: () => void
}

const BreadcrumbContext = createContext<BreadcrumbContextType | undefined>(undefined)

interface BreadcrumbProviderProps {
  children: ReactNode
}

export function BreadcrumbProvider({ children }: BreadcrumbProviderProps) {
  const [items, setItems] = useState<BreadcrumbItem[]>([])

  const addItem = (item: BreadcrumbItem) => {
    setItems(prev => [...prev, item])
  }

  const clearItems = () => {
    setItems([])
  }

  return (
    <BreadcrumbContext.Provider value={{ items, setItems, addItem, clearItems }}>
      {children}
    </BreadcrumbContext.Provider>
  )
}

export function useBreadcrumb() {
  const context = useContext(BreadcrumbContext)
  if (context === undefined) {
    throw new Error('useBreadcrumb must be used within a BreadcrumbProvider')
  }
  return context
}
