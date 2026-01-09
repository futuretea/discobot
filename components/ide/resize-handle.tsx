"use client"

import * as React from "react"
import { cn } from "@/lib/utils"

interface ResizeHandleProps {
  onResize: (delta: number) => void
  className?: string
}

export function ResizeHandle({ onResize, className }: ResizeHandleProps) {
  const [isDragging, setIsDragging] = React.useState(false)
  const startYRef = React.useRef(0)

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    setIsDragging(true)
    startYRef.current = e.clientY
  }

  React.useEffect(() => {
    if (!isDragging) return

    const handleMouseMove = (e: MouseEvent) => {
      const delta = e.clientY - startYRef.current
      startYRef.current = e.clientY
      onResize(delta)
    }

    const handleMouseUp = () => {
      setIsDragging(false)
    }

    document.addEventListener("mousemove", handleMouseMove)
    document.addEventListener("mouseup", handleMouseUp)

    return () => {
      document.removeEventListener("mousemove", handleMouseMove)
      document.removeEventListener("mouseup", handleMouseUp)
    }
  }, [isDragging, onResize])

  return (
    <div
      className={cn(
        "h-1 cursor-row-resize hover:bg-primary/20 transition-colors",
        isDragging && "bg-primary/30",
        className,
      )}
      onMouseDown={handleMouseDown}
    />
  )
}
