"use client"

import * as React from "react"
import {
  RefreshCcw,
  Copy,
  Terminal,
  MessageSquare,
  Plus,
  ChevronDown,
  HardDrive,
  GitBranch,
  Bot,
  Circle,
} from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import type { ChatMessage, Workspace, Agent } from "@/lib/mock-data"

import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation"
import {
  Message,
  MessageContent,
  MessageResponse,
  MessageActions,
  MessageAction,
  MessageRoleProvider,
} from "@/components/ai-elements/message"
import {
  Input,
  PromptInputTextarea,
  PromptInputSubmit,
  PromptInputToolbar,
  PromptInputTools,
} from "@/components/ai-elements/prompt-input"

function getWorkspaceType(path: string): "github" | "git" | "local" {
  if (path.includes("github.com") || path.startsWith("git@github.com")) {
    return "github"
  }
  if (path.startsWith("git@") || path.startsWith("git://") || (path.startsWith("https://") && path.includes(".git"))) {
    return "git"
  }
  return "local"
}

function getWorkspaceDisplayName(path: string): string {
  const type = getWorkspaceType(path)
  if (type === "github") {
    const match = path.match(/github\.com[:/](.+?)(\.git)?$/)
    if (match) return match[1].replace(/\.git$/, "")
    return path
  }
  if (type === "git") {
    return path.replace(/^(git@|git:\/\/|https?:\/\/)/, "").replace(/\.git$/, "")
  }
  return path
}

function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.31.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  )
}

function WorkspaceIcon({ path, className }: { path: string; className?: string }) {
  const type = getWorkspaceType(path)
  if (type === "github") return <GitHubIcon className={className} />
  if (type === "git") return <GitBranch className={cn("text-orange-500", className)} />
  return <HardDrive className={cn("text-blue-500", className)} />
}

interface ChatPanelProps {
  initialMessages?: ChatMessage[]
  onToggleTerminal: () => void
  showTerminal: boolean
  className?: string
  hideHeader?: boolean
  centered?: boolean
  onFirstMessage?: (message: string, workspaceId: string, agentId: string) => void
  workspaces?: Workspace[]
  selectedWorkspaceId?: string | null
  onAddWorkspace?: () => void
  workspaceSelectTrigger?: number
  agents?: Agent[]
  selectedAgentId?: string | null
}

export function ChatPanel({
  initialMessages = [],
  onToggleTerminal,
  showTerminal,
  className,
  hideHeader,
  centered,
  onFirstMessage,
  workspaces = [],
  selectedWorkspaceId,
  onAddWorkspace,
  workspaceSelectTrigger,
  agents = [],
  selectedAgentId,
}: ChatPanelProps) {
  const [input, setInput] = React.useState("")
  const [isLoading, setIsLoading] = React.useState(false)
  const [localSelectedWorkspaceId, setLocalSelectedWorkspaceId] = React.useState<string | null>(
    selectedWorkspaceId || (workspaces.length > 0 ? workspaces[0].id : null),
  )
  const [localSelectedAgentId, setLocalSelectedAgentId] = React.useState<string | null>(
    selectedAgentId || (agents.length > 0 ? agents[0].id : null),
  )
  const [isShimmering, setIsShimmering] = React.useState(false)

  React.useEffect(() => {
    if (selectedWorkspaceId) {
      setLocalSelectedWorkspaceId(selectedWorkspaceId)
    }
  }, [selectedWorkspaceId])

  React.useEffect(() => {
    if (selectedAgentId) {
      setLocalSelectedAgentId(selectedAgentId)
    }
  }, [selectedAgentId])

  React.useEffect(() => {
    if (workspaceSelectTrigger && workspaceSelectTrigger > 0) {
      setIsShimmering(true)
      const timeout = setTimeout(() => setIsShimmering(false), 600)
      return () => clearTimeout(timeout)
    }
  }, [workspaceSelectTrigger])

  const selectedWorkspace = workspaces.find((ws) => ws.id === localSelectedWorkspaceId)
  const selectedAgent = agents.find((a) => a.id === localSelectedAgentId)

  const [messages, setMessages] = React.useState<InternalMessage[]>(() =>
    initialMessages.map((m) => ({
      id: m.id,
      role: m.role,
      parts: [{ type: "text" as const, text: m.content }],
      createdAt: new Date(),
    })),
  )

  React.useEffect(() => {
    setMessages(
      initialMessages.map((m) => ({
        id: m.id,
        role: m.role,
        parts: [{ type: "text", text: m.content }],
        createdAt: new Date(),
      })),
    )
  }, [initialMessages])

  const groupedByTurn = React.useMemo(() => {
    const groups: { turn: number; messages: InternalMessage[] }[] = []
    let currentTurn = 1
    let currentGroup: InternalMessage[] = []

    messages.forEach((msg) => {
      currentGroup.push(msg)
      if (msg.role === "assistant") {
        groups.push({ turn: currentTurn, messages: currentGroup })
        currentGroup = []
        currentTurn++
      }
    })

    if (currentGroup.length > 0) {
      groups.push({ turn: currentTurn, messages: currentGroup })
    }

    return groups
  }, [messages])

  const handleSubmit = async () => {
    if (!input.trim() || isLoading) return

    const messageText = input

    if (messages.length === 0 && onFirstMessage && localSelectedWorkspaceId && localSelectedAgentId) {
      onFirstMessage(messageText, localSelectedWorkspaceId, localSelectedAgentId)
    }

    const userMessage: InternalMessage = {
      id: `msg-${Date.now()}`,
      role: "user",
      parts: [{ type: "text", text: messageText }],
      createdAt: new Date(),
    }

    setMessages((prev) => [...prev, userMessage])
    setInput("")
    setIsLoading(true)

    await new Promise((resolve) => setTimeout(resolve, 1000))

    const assistantMessage: InternalMessage = {
      id: `msg-${Date.now() + 1}`,
      role: "assistant",
      parts: [
        {
          type: "text",
          text: `I understand you're asking about: "${messageText}"\n\nThis is a simulated response. In production, this would connect to your AI backend via the API route.`,
        },
      ],
      createdAt: new Date(),
    }

    setMessages((prev) => [...prev, assistantMessage])
    setIsLoading(false)
  }

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
  }

  const handleRegenerate = () => {
    console.log("Regenerate last response")
  }

  const status = isLoading ? "streaming" : "ready"

  if (centered) {
    return (
      <div className={cn("flex flex-col items-center justify-center h-full bg-background p-8", className)}>
        <div className="w-full max-w-2xl space-y-6">
          <div className="text-center space-y-2">
            <MessageSquare className="h-12 w-12 mx-auto text-muted-foreground/50" />
            <h2 className="text-xl font-semibold">Start a new session</h2>
            <p className="text-muted-foreground text-sm">
              Describe what you want to work on and I'll help you get started.
            </p>
          </div>

          <div className="flex flex-col items-center gap-3">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground w-20 text-right">Agent:</span>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" className="gap-2 min-w-[200px] justify-between bg-transparent">
                    {selectedAgent ? (
                      <>
                        <div className="flex items-center gap-2 truncate">
                          <Bot className="h-4 w-4 shrink-0 text-muted-foreground" />
                          <span className="truncate">{selectedAgent.name}</span>
                          <Circle
                            className={cn(
                              "h-2 w-2 shrink-0",
                              selectedAgent.status === "active"
                                ? "fill-green-500 text-green-500"
                                : "fill-muted-foreground text-muted-foreground",
                            )}
                          />
                        </div>
                        <ChevronDown className="h-4 w-4 shrink-0 opacity-50" />
                      </>
                    ) : (
                      <>
                        <span className="text-muted-foreground">Select agent</span>
                        <ChevronDown className="h-4 w-4 shrink-0 opacity-50" />
                      </>
                    )}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="center" className="w-[250px]">
                  {agents.map((agent) => (
                    <DropdownMenuItem
                      key={agent.id}
                      onClick={() => setLocalSelectedAgentId(agent.id)}
                      className="gap-2"
                    >
                      <Bot className="h-4 w-4 shrink-0 text-muted-foreground" />
                      <span className="truncate flex-1">{agent.name}</span>
                      <Circle
                        className={cn(
                          "h-2 w-2 shrink-0",
                          agent.status === "active"
                            ? "fill-green-500 text-green-500"
                            : "fill-muted-foreground text-muted-foreground",
                        )}
                      />
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>

            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground w-20 text-right">Workspace:</span>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="outline"
                    size="sm"
                    className={cn(
                      "gap-2 min-w-[200px] justify-between bg-transparent transition-all",
                      isShimmering && "animate-pulse ring-2 ring-primary/50",
                    )}
                  >
                    {selectedWorkspace ? (
                      <>
                        <div className="flex items-center gap-2 truncate">
                          <WorkspaceIcon path={selectedWorkspace.path} className="h-4 w-4 shrink-0" />
                          <span className="truncate">{getWorkspaceDisplayName(selectedWorkspace.path)}</span>
                        </div>
                        <ChevronDown className="h-4 w-4 shrink-0 opacity-50" />
                      </>
                    ) : (
                      <>
                        <span className="text-muted-foreground">Select workspace</span>
                        <ChevronDown className="h-4 w-4 shrink-0 opacity-50" />
                      </>
                    )}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="center" className="w-[250px]">
                  {workspaces.map((ws) => (
                    <DropdownMenuItem key={ws.id} onClick={() => setLocalSelectedWorkspaceId(ws.id)} className="gap-2">
                      <WorkspaceIcon path={ws.path} className="h-4 w-4 shrink-0" />
                      <span className="truncate">{getWorkspaceDisplayName(ws.path)}</span>
                    </DropdownMenuItem>
                  ))}
                  {workspaces.length > 0 && <DropdownMenuSeparator />}
                  <DropdownMenuItem onClick={onAddWorkspace} className="gap-2">
                    <Plus className="h-4 w-4" />
                    <span>Add Workspace</span>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>

          <Input onSubmit={handleSubmit} value={input} onChange={setInput} status={status} className="max-w-full">
            <PromptInputTextarea placeholder="What would you like to work on?" className="min-h-[80px] text-base" />
            <PromptInputToolbar>
              <PromptInputTools />
              <PromptInputSubmit status={status} disabled={!localSelectedWorkspaceId || !localSelectedAgentId} />
            </PromptInputToolbar>
          </Input>
        </div>
      </div>
    )
  }

  return (
    <div className={cn("flex flex-col h-full bg-background", className)}>
      {!hideHeader && (
        <div className="flex items-center justify-between px-4 py-2 border-b border-border shrink-0">
          <h2 className="font-medium text-sm">Chat</h2>
          <Button variant={showTerminal ? "default" : "ghost"} size="sm" onClick={onToggleTerminal} className="gap-2">
            <Terminal className="h-4 w-4" />
            <span className="hidden sm:inline">Terminal</span>
          </Button>
        </div>
      )}

      <Conversation className="flex-1 min-h-0">
        <ConversationContent className="p-4">
          {groupedByTurn.length === 0 ? (
            <ConversationEmptyState
              icon={<MessageSquare className="size-12 opacity-50" />}
              title="Start a conversation"
              description="Type a message below to begin chatting with the AI assistant."
            />
          ) : (
            <div className="space-y-6">
              {groupedByTurn.map((group, groupIdx) => (
                <div key={groupIdx} className="relative">
                  <div className="flex items-center gap-2 mb-3">
                    <span className="text-xs font-medium text-muted-foreground bg-muted px-2 py-0.5 rounded">
                      Turn {group.turn}
                    </span>
                    <div className="flex-1 h-px bg-border" />
                  </div>

                  <div className="space-y-3 pl-3 border-l-2 border-border">
                    {group.messages.map((message, messageIdx) => (
                      <MessageRoleProvider key={message.id} role={message.role}>
                        <Message from={message.role}>
                          <MessageContent>
                            <div className="text-xs font-medium text-muted-foreground mb-1">
                              {message.role === "user" ? "You" : "Assistant"}
                            </div>
                            {message.parts.map((part, i) => {
                              if (part.type === "text") {
                                return <MessageResponse key={`${message.id}-${i}`}>{part.text}</MessageResponse>
                              }
                              return null
                            })}
                            {message.role === "assistant" && messageIdx === group.messages.length - 1 && (
                              <MessageActions>
                                <MessageAction label="Retry" tooltip="Regenerate response" onClick={handleRegenerate}>
                                  <RefreshCcw className="size-3" />
                                </MessageAction>
                                <MessageAction
                                  label="Copy"
                                  tooltip="Copy to clipboard"
                                  onClick={() => {
                                    const textPart = message.parts.find((p) => p.type === "text")
                                    if (textPart && "text" in textPart) {
                                      handleCopy(textPart.text)
                                    }
                                  }}
                                >
                                  <Copy className="size-3" />
                                </MessageAction>
                              </MessageActions>
                            )}
                          </MessageContent>
                        </Message>
                      </MessageRoleProvider>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </ConversationContent>
        <ConversationScrollButton />
      </Conversation>

      <div className="p-4 border-t border-border shrink-0">
        <Input onSubmit={handleSubmit} value={input} onChange={setInput} status={status} className="max-w-full">
          <PromptInputTextarea
            placeholder="Type a message... (Enter to send, Shift+Enter for new line)"
            className="min-h-[44px]"
          />
          <PromptInputToolbar>
            <PromptInputTools />
            <PromptInputSubmit status={status} />
          </PromptInputToolbar>
        </Input>
      </div>
    </div>
  )
}

interface InternalMessage {
  id: string
  role: "user" | "assistant"
  parts: { type: "text"; text: string }[]
  createdAt: Date
}
