"use client"

import * as React from "react"
import { MessageSquare } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { mockTerminalLines } from "@/lib/mock-data"

interface TerminalViewProps {
  className?: string
  onToggleChat?: () => void
  hideHeader?: boolean
}

export function TerminalView({ className, onToggleChat, hideHeader }: TerminalViewProps) {
  const [lines, setLines] = React.useState(mockTerminalLines)
  const [input, setInput] = React.useState("")
  const terminalRef = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    terminalRef.current?.scrollTo(0, terminalRef.current.scrollHeight)
  }, [lines])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim()) return

    setLines((prev) => [
      ...prev,
      { text: `$ ${input}`, type: "command" as const },
      { text: `Command '${input}' executed (simulated)`, type: "output" as const },
      { text: "", type: "output" as const },
      { text: "user@dev-server:~$ ", type: "prompt" as const },
    ])
    setInput("")
  }

  return (
    <div className={cn("flex flex-col h-full bg-terminal-bg text-terminal-fg font-mono text-sm", className)}>
      {!hideHeader && (
        <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-background">
          <div className="flex items-center gap-2">
            <div className="flex gap-1.5">
              <div className="w-3 h-3 rounded-full bg-red-500" />
              <div className="w-3 h-3 rounded-full bg-yellow-500" />
              <div className="w-3 h-3 rounded-full bg-green-500" />
            </div>
            <span className="text-xs text-muted-foreground ml-2">SSH: user@dev-server.local</span>
          </div>
          {onToggleChat && (
            <Button variant="ghost" size="sm" onClick={onToggleChat} className="gap-2">
              <MessageSquare className="h-4 w-4" />
              <span className="hidden sm:inline">Back to Chat</span>
            </Button>
          )}
        </div>
      )}

      <div ref={terminalRef} className="flex-1 overflow-y-auto p-4 space-y-0.5">
        {lines.map((line, idx) => (
          <div
            key={idx}
            className={cn(
              "leading-relaxed",
              line.type === "command" && "text-foreground",
              line.type === "success" && "text-green-400",
              line.type === "modified" && "text-yellow-400",
              line.type === "prompt" && "text-terminal-fg",
            )}
          >
            {line.text || "\u00A0"}
          </div>
        ))}
      </div>

      <form onSubmit={handleSubmit} className="p-4 border-t border-border/30">
        <div className="flex items-center gap-2">
          <span className="text-terminal-fg">$</span>
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            className="flex-1 bg-transparent outline-none text-foreground placeholder:text-muted-foreground"
            placeholder="Enter command..."
            autoFocus
          />
        </div>
      </form>
    </div>
  )
}
