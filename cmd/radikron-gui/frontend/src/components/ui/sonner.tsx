"use client"

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
  const getEffectiveTheme = useThemeStore((state) => state.getEffectiveTheme)
  const effectiveTheme = getEffectiveTheme()

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
