import * as React from "react"

interface TabsContextValue {
  value: string
  onValueChange: (value: string) => void
  registerTab: (value: string) => void
  unregisterTab: (value: string) => void
  tabValues: string[]
}

const TabsContext = React.createContext<TabsContextValue | undefined>(undefined)

interface TabsProps {
  defaultValue?: string
  value?: string
  onValueChange?: (value: string) => void
  children: React.ReactNode
  className?: string
}

export const Tabs: React.FC<TabsProps> = ({
  defaultValue,
  value: controlledValue,
  onValueChange: controlledOnValueChange,
  children,
  className = "",
}) => {
  const [uncontrolledValue, setUncontrolledValue] = React.useState(defaultValue || "")
  const [tabValues, setTabValues] = React.useState<string[]>([])
  const isControlled = controlledValue !== undefined
  const value = isControlled ? controlledValue : uncontrolledValue
  const onValueChange = isControlled ? controlledOnValueChange : setUncontrolledValue

  const registerTab = React.useCallback((tabValue: string) => {
    setTabValues((prev) => {
      if (!prev.includes(tabValue)) {
        return [...prev, tabValue]
      }
      return prev
    })
  }, [])

  const unregisterTab = React.useCallback((tabValue: string) => {
    setTabValues((prev) => prev.filter((v) => v !== tabValue))
  }, [])

  return (
    <TabsContext.Provider
      value={{
        value: value || "",
        onValueChange: onValueChange || (() => {}),
        registerTab,
        unregisterTab,
        tabValues,
      }}
    >
      <div className={className}>{children}</div>
    </TabsContext.Provider>
  )
}

interface TabsListProps {
  children: React.ReactNode
  className?: string
}

export const TabsList: React.FC<TabsListProps> = ({ children, className = "" }) => {
  return (
    <div
      role="tablist"
      className={`inline-flex h-10 items-center justify-center rounded-md bg-muted p-1 text-muted-foreground ${className}`}
    >
      {children}
    </div>
  )
}

interface TabsTriggerProps {
  value: string
  children: React.ReactNode
  className?: string
}

export const TabsTrigger: React.FC<TabsTriggerProps> = ({ value, children, className = "" }) => {
  const context = React.useContext(TabsContext)

  if (!context) {
    throw new Error("TabsTrigger must be used within Tabs")
  }

  const isActive = context.value === value
  const tabId = `tab-${value}`
  const panelId = `panel-${value}`
  const { registerTab, unregisterTab } = context

  // Register this tab when component mounts
  React.useEffect(() => {
    registerTab(value)
    return () => {
      unregisterTab(value)
    }
  }, [value, registerTab, unregisterTab])

  const getTabIndex = (currentValue: string): number => {
    return context.tabValues.indexOf(currentValue)
  }

  const focusNextTab = () => {
    const currentIndex = getTabIndex(value)
    const nextIndex = (currentIndex + 1) % context.tabValues.length
    const nextValue = context.tabValues[nextIndex]
    context.onValueChange(nextValue)
    // Focus will be managed by the next tab's button
    setTimeout(() => {
      const nextButton = document.getElementById(`tab-${nextValue}`) as HTMLButtonElement
      if (nextButton) {
        nextButton.focus()
      }
    }, 0)
  }

  const focusPrevTab = () => {
    const currentIndex = getTabIndex(value)
    const prevIndex = currentIndex === 0 ? context.tabValues.length - 1 : currentIndex - 1
    const prevValue = context.tabValues[prevIndex]
    context.onValueChange(prevValue)
    setTimeout(() => {
      const prevButton = document.getElementById(`tab-${prevValue}`) as HTMLButtonElement
      if (prevButton) {
        prevButton.focus()
      }
    }, 0)
  }

  const focusFirstTab = () => {
    if (context.tabValues.length > 0) {
      const firstValue = context.tabValues[0]
      context.onValueChange(firstValue)
      setTimeout(() => {
        const firstButton = document.getElementById(`tab-${firstValue}`) as HTMLButtonElement
        if (firstButton) {
          firstButton.focus()
        }
      }, 0)
    }
  }

  const focusLastTab = () => {
    if (context.tabValues.length > 0) {
      const lastValue = context.tabValues[context.tabValues.length - 1]
      context.onValueChange(lastValue)
      setTimeout(() => {
        const lastButton = document.getElementById(`tab-${lastValue}`) as HTMLButtonElement
        if (lastButton) {
          lastButton.focus()
        }
      }, 0)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    switch (e.key) {
      case "ArrowRight":
        e.preventDefault()
        focusNextTab()
        break
      case "ArrowLeft":
        e.preventDefault()
        focusPrevTab()
        break
      case "Home":
        e.preventDefault()
        focusFirstTab()
        break
      case "End":
        e.preventDefault()
        focusLastTab()
        break
      default:
        break
    }
  }

  return (
    <button
      type="button"
      role="tab"
      id={tabId}
      aria-selected={isActive}
      aria-controls={panelId}
      tabIndex={isActive ? 0 : -1}
      onClick={() => context.onValueChange(value)}
      onKeyDown={handleKeyDown}
      className={`inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 ${
        isActive
          ? "bg-background text-foreground shadow-sm"
          : "text-muted-foreground hover:bg-background/50"
      } ${className}`}
    >
      {children}
    </button>
  )
}

interface TabsContentProps {
  value: string
  children: React.ReactNode
  className?: string
}

export const TabsContent: React.FC<TabsContentProps> = ({ value, children, className = "" }) => {
  const context = React.useContext(TabsContext)
  if (!context) {
    throw new Error("TabsContent must be used within Tabs")
  }

  const isActive = context.value === value
  const tabId = `tab-${value}`
  const panelId = `panel-${value}`

  if (!isActive) {
    return null
  }

  return (
    <div
      role="tabpanel"
      id={panelId}
      aria-labelledby={tabId}
      className={`mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ${className}`}
    >
      {children}
    </div>
  )
}
