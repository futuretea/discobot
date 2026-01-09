"use client"

import * as React from "react"
import { HardDrive, GitBranch, AlertCircle, Check } from "lucide-react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"
import type { Workspace } from "@/lib/mock-data"

function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  )
}

type InputType = "unknown" | "local" | "git" | "github"

interface ValidationResult {
  isValid: boolean
  type: InputType
  error?: string
}

// Mock suggestions for autocomplete
const mockGitHubSuggestions = [
  "github.com/vercel/next.js",
  "github.com/vercel/ai",
  "github.com/facebook/react",
  "github.com/microsoft/typescript",
  "github.com/tailwindlabs/tailwindcss",
  "github.com/shadcn-ui/ui",
]

const mockLocalSuggestions = [
  "~/projects/my-app",
  "~/projects/website",
  "~/code/api-server",
  "/home/user/dev/dashboard",
  "/var/www/production",
  "C:\\Users\\dev\\projects\\app",
]

function detectInputType(input: string): InputType {
  if (!input.trim()) return "unknown"

  const trimmed = input.trim()

  // GitHub patterns
  if (
    trimmed.match(/^(https?:\/\/)?(www\.)?github\.com\//) ||
    trimmed.match(/^git@github\.com:/) ||
    trimmed.match(/^github\.com\//)
  ) {
    return "github"
  }

  // Generic git patterns
  if (
    trimmed.match(/^(https?:\/\/|git@|ssh:\/\/|git:\/\/)/) ||
    trimmed.match(/\.git$/) ||
    trimmed.match(/^[a-z]+@[a-z0-9.-]+:/)
  ) {
    return "git"
  }

  // Local path patterns
  if (
    trimmed.startsWith("~") ||
    trimmed.startsWith("/") ||
    trimmed.startsWith("./") ||
    trimmed.match(/^[A-Z]:\\/) || // Windows path
    trimmed.match(/^\.\.\//)
  ) {
    return "local"
  }

  return "unknown"
}

function validateInput(input: string): ValidationResult {
  const type = detectInputType(input)

  if (!input.trim()) {
    return { isValid: false, type: "unknown", error: undefined }
  }

  if (type === "unknown") {
    return {
      isValid: false,
      type: "unknown",
      error: "Enter a valid path (e.g., ~/projects/app) or git URL (e.g., github.com/org/repo)",
    }
  }

  if (type === "github") {
    // Validate GitHub URL format
    const match = input.match(/github\.com[/:]([\w-]+)\/([\w.-]+)/)
    if (!match) {
      return {
        isValid: false,
        type: "github",
        error: "Invalid GitHub URL. Use format: github.com/org/repo",
      }
    }
    return { isValid: true, type: "github" }
  }

  if (type === "git") {
    // Basic git URL validation
    if (!input.match(/[\w-]+\/[\w.-]+/) && !input.match(/\.git$/)) {
      return {
        isValid: false,
        type: "git",
        error: "Invalid git URL format",
      }
    }
    return { isValid: true, type: "git" }
  }

  if (type === "local") {
    // Basic path validation
    if (input.length < 2) {
      return {
        isValid: false,
        type: "local",
        error: "Path too short",
      }
    }
    return { isValid: true, type: "local" }
  }

  return { isValid: false, type: "unknown" }
}

function getInputIcon(type: InputType, className?: string) {
  switch (type) {
    case "github":
      return <GitHubIcon className={className} />
    case "git":
      return <GitBranch className={cn(className, "text-orange-500")} />
    case "local":
      return <HardDrive className={cn(className, "text-blue-500")} />
    default:
      return <HardDrive className={cn(className, "text-muted-foreground")} />
  }
}

interface AddWorkspaceDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdd: (workspace: Omit<Workspace, "id" | "sessions">) => void
}

export function AddWorkspaceDialog({ open, onOpenChange, onAdd }: AddWorkspaceDialogProps) {
  const [input, setInput] = React.useState("")
  const [showSuggestions, setShowSuggestions] = React.useState(false)
  const [selectedIndex, setSelectedIndex] = React.useState(-1)
  const inputRef = React.useRef<HTMLInputElement>(null)

  const validation = validateInput(input)
  const inputType = detectInputType(input)

  // Filter suggestions based on input
  const suggestions = React.useMemo(() => {
    if (!input.trim()) return []

    const lower = input.toLowerCase()
    const allSuggestions = [...mockGitHubSuggestions, ...mockLocalSuggestions]

    return allSuggestions.filter((s) => s.toLowerCase().includes(lower)).slice(0, 6)
  }, [input])

  React.useEffect(() => {
    setSelectedIndex(-1)
  }, [suggestions])

  const handleSubmit = () => {
    if (!validation.isValid) return

    const sourceType = inputType === "local" ? "local" : "git"
    onAdd({
      name: input.split("/").pop() || input,
      path: input.trim(),
      sourceType,
    })

    setInput("")
    onOpenChange(false)
  }

  const handleSuggestionClick = (suggestion: string) => {
    setInput(suggestion)
    setShowSuggestions(false)
    setSelectedIndex(-1)
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || suggestions.length === 0) {
      if (e.key === "Enter" && validation.isValid) {
        handleSubmit()
      }
      return
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault()
        setSelectedIndex((prev) => (prev < suggestions.length - 1 ? prev + 1 : prev))
        break
      case "ArrowUp":
        e.preventDefault()
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1))
        break
      case "Enter":
        e.preventDefault()
        if (selectedIndex >= 0 && selectedIndex < suggestions.length) {
          handleSuggestionClick(suggestions[selectedIndex])
        } else if (validation.isValid) {
          handleSubmit()
        }
        break
      case "Escape":
        setShowSuggestions(false)
        setSelectedIndex(-1)
        break
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add Workspace</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="relative">
            <div className="flex items-center gap-2">
              <div className="shrink-0 w-8 h-8 flex items-center justify-center rounded-md border bg-muted">
                {getInputIcon(inputType, "h-4 w-4")}
              </div>
              <div className="flex-1 relative">
                <Input
                  ref={inputRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onFocus={() => setShowSuggestions(true)}
                  onBlur={() => setTimeout(() => setShowSuggestions(false), 150)}
                  placeholder="~/projects/app or github.com/org/repo"
                  className={cn(
                    "font-mono text-sm",
                    validation.error && input.trim() && "border-destructive focus-visible:ring-destructive",
                  )}
                  onKeyDown={handleKeyDown}
                />
                {/* Suggestions dropdown */}
                {showSuggestions && suggestions.length > 0 && (
                  <div className="absolute z-50 top-full left-0 right-0 mt-1 bg-popover border rounded-md shadow-lg max-h-48 overflow-y-auto">
                    {suggestions.map((suggestion, index) => {
                      const suggestionType = detectInputType(suggestion)
                      return (
                        <button
                          key={suggestion}
                          className={cn(
                            "w-full flex items-center gap-2 px-3 py-2 text-sm font-mono hover:bg-accent text-left",
                            index === selectedIndex && "bg-accent",
                          )}
                          onMouseDown={() => handleSuggestionClick(suggestion)}
                          onMouseEnter={() => setSelectedIndex(index)}
                        >
                          {getInputIcon(suggestionType, "h-3.5 w-3.5 shrink-0")}
                          <span className="truncate">{suggestion}</span>
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            </div>
            <div className="mt-3 h-5 flex items-center">
              {input.trim() && (
                <div className="flex items-center gap-1.5 text-xs">
                  {validation.isValid ? (
                    <>
                      <Check className="h-3.5 w-3.5 text-green-500" />
                      <span className="text-muted-foreground">
                        {inputType === "github" && "GitHub repository"}
                        {inputType === "git" && "Git repository"}
                        {inputType === "local" && "Local folder"}
                      </span>
                    </>
                  ) : (
                    <>
                      <AlertCircle className="h-3.5 w-3.5 text-destructive" />
                      <span className="text-destructive">{validation.error}</span>
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
          <div className="text-xs text-muted-foreground space-y-1">
            <p className="font-medium">Supported formats:</p>
            <ul className="list-disc list-inside space-y-0.5 pl-1">
              <li>Local paths: ~/projects/app, /var/www/site</li>
              <li>GitHub: github.com/org/repo, git@github.com:org/repo</li>
              <li>Git: https://gitlab.com/org/repo, git@bitbucket.org:org/repo</li>
            </ul>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSubmit} disabled={!validation.isValid}>
            Add Workspace
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
