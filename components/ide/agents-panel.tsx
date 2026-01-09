"use client"

import * as React from "react"
import { Bot, Circle, Plus, MoreHorizontal, ChevronUp, ChevronDown } from "lucide-react"
import { cn } from "@/lib/utils"
import type { Agent } from "@/lib/mock-data"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"

interface AgentsPanelProps {
  agents: Agent[]
  selectedAgentId: string | null
  onAgentSelect: (agent: Agent) => void
  onAddAgent?: () => void
  isMinimized: boolean
  onToggleMinimize: () => void
  className?: string
  style?: React.CSSProperties // Add style prop
}

export function AgentsPanel({
  agents,
  selectedAgentId,
  onAgentSelect,
  onAddAgent,
  isMinimized,
  onToggleMinimize,
  className,
  style, // Accept style prop
}: AgentsPanelProps) {
  return (
    <div
      className={cn("flex flex-col overflow-hidden border-t border-sidebar-border", className)}
      style={style} // Apply style
    >
      <div
        className="px-3 py-2 flex items-center justify-between cursor-pointer hover:bg-sidebar-accent"
        onClick={onToggleMinimize}
      >
        <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Agents</span>
        <div className="flex items-center gap-1">
          {onAddAgent && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                onAddAgent()
              }}
              className="p-1 rounded hover:bg-sidebar-accent transition-colors"
              title="Add agent"
            >
              <Plus className="h-3.5 w-3.5 text-muted-foreground" />
            </button>
          )}
          <button className="p-1 rounded hover:bg-sidebar-accent transition-colors">
            {isMinimized ? (
              <ChevronUp className="h-3.5 w-3.5 text-muted-foreground" />
            ) : (
              <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
            )}
          </button>
        </div>
      </div>
      {!isMinimized && (
        <div className="flex-1 overflow-y-auto py-1">
          {agents.map((agent) => (
            <AgentNode
              key={agent.id}
              agent={agent}
              isSelected={selectedAgentId === agent.id}
              onSelect={() => onAgentSelect(agent)}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function AgentNode({
  agent,
  isSelected,
  onSelect,
}: {
  agent: Agent
  isSelected: boolean
  onSelect: () => void
}) {
  const [menuOpen, setMenuOpen] = React.useState(false)

  return (
    <div
      className={cn(
        "group flex items-center gap-1.5 px-2 py-1 hover:bg-sidebar-accent cursor-pointer transition-colors",
        isSelected && "bg-sidebar-accent",
        agent.status === "inactive" && "opacity-60",
      )}
      onClick={onSelect}
    >
      <Bot className="h-4 w-4 text-muted-foreground shrink-0" />
      <div className="flex-1 min-w-0">
        <span className="text-sm truncate block">{agent.name}</span>
      </div>
      <Circle
        className={cn(
          "h-2 w-2 shrink-0",
          agent.status === "active" ? "fill-green-500 text-green-500" : "fill-muted-foreground text-muted-foreground",
        )}
      />
      <DropdownMenu open={menuOpen} onOpenChange={setMenuOpen}>
        <DropdownMenuTrigger asChild>
          <button
            onClick={(e) => e.stopPropagation()}
            className={cn(
              "p-0.5 rounded hover:bg-muted shrink-0",
              menuOpen ? "opacity-100" : "opacity-0 group-hover:opacity-100",
            )}
          >
            <MoreHorizontal className="h-4 w-4 text-muted-foreground" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-32">
          <DropdownMenuItem>Configure</DropdownMenuItem>
          <DropdownMenuItem>Duplicate</DropdownMenuItem>
          <DropdownMenuItem className="text-destructive">Delete</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
