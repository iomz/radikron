"use client"

import { useEffect, useState } from "react"
import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useThemeStore } from "@/store/useThemeStore"
import { Toaster as Sonner, type ToasterProps } from "sonner"

const Toaster = ({ ...props }: ToasterProps) => {
  const theme = useThemeStore((state) => state.theme)
  const [effectiveTheme, setEffectiveTheme] = useState<"light" | "dark">(() => {
    if (theme === "system") {
      if (typeof window === "undefined") return "light"
      return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
    }
    return theme
  })

  useEffect(() => {
    const computeEffectiveTheme = () => {
      if (theme === "system") {
        if (typeof window === "undefined") return "light"
        return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
      }
      return theme
    }

    setEffectiveTheme(computeEffectiveTheme())

    if (theme === "system" && typeof window !== "undefined") {
      const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)")
      const handleChange = () => {
        setEffectiveTheme(computeEffectiveTheme())
      }

      if (mediaQuery.addEventListener) {
        mediaQuery.addEventListener("change", handleChange)
        return () => mediaQuery.removeEventListener("change", handleChange)
      } else {
        mediaQuery.addListener(handleChange)
        return () => mediaQuery.removeListener(handleChange)
      }
    }
  }, [theme])

  return (
    <Sonner
      theme={effectiveTheme as ToasterProps["theme"]}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      toastOptions={{
        style: {
          background: effectiveTheme === "dark" 
            ? "oklch(20% 0 0)" // Lighter background for better visibility in dark mode
            : undefined,
          color: effectiveTheme === "dark"
            ? "oklch(98% 0 0)" // Ensure high contrast text
            : undefined,
          border: `1px solid ${effectiveTheme === "dark" ? "oklch(30% 0 0)" : "var(--color-border)"}`,
        },
        className: effectiveTheme === "dark" ? "dark-toast" : "",
      }}
      style={
        {
          "--normal-bg": effectiveTheme === "dark" 
            ? "oklch(20% 0 0)" // Lighter background for better visibility
            : "var(--color-popover)",
          "--normal-text": effectiveTheme === "dark"
            ? "oklch(98% 0 0)" // High contrast text
            : "var(--color-popover-foreground)",
          "--normal-border": effectiveTheme === "dark"
            ? "oklch(30% 0 0)" // Visible border
            : "var(--color-border)",
          "--border-radius": "var(--radius)",
        } as React.CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
