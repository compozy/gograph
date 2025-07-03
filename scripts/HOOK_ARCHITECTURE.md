# gograph Claude Code Hook Architecture

## Overview

This hook implements Claude Code's PreToolUse hook system to provide intelligent Go code analysis before file operations.

## Implementation Details

### Hook Configuration Format

The hook uses the correct Claude Code hooks structure:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "cd /path/to/gograph && bun run scripts/gograph-hook.ts"
          }
        ]
      }
    ]
  }
}
```

### Input Processing

The hook receives JSON input via stdin with this structure:

```typescript
interface PreToolUseInput {
  session_id: string;
  transcript_path: string;
  tool_name: string;
  tool_input: Record<string, any>;
}
```

### Output Format

The hook returns structured JSON output for advanced control:

```typescript
interface HookOutput {
  // Common fields
  continue?: boolean;       // Whether Claude should continue (default: true)
  stopReason?: string;      // Message shown when continue is false
  suppressOutput?: boolean; // Hide stdout from transcript mode (default: false)
  
  // PreToolUse specific
  decision?: "approve" | "block" | undefined;
  reason?: string;          // Explanation for decision
}
```

### Decision Logic

- **Approve**: The hook always approves Go file operations but provides analysis
- **Reason**: Contains the formatted analysis output shown to Claude
- **Continue**: Always true to allow operation to proceed
- **Exit Codes**: 
  - 0: Success with JSON output
  - 1: Non-blocking error (operation continues)
  - 2: Would block operation (not used in our implementation)

## Features

### 1. Automatic Go File Detection
- Checks `file_path` parameter for `.go` extension
- Handles MultiEdit operations with multiple edits
- Skips non-Go files silently

### 2. gograph Configuration Detection
- Looks for `gograph.yaml` in project hierarchy
- Provides appropriate analysis based on configuration presence

### 3. Claude SDK Integration
- Uses existing Claude Code session (no API key needed)
- Includes `allowedTools` for MCP permissions
- Graceful fallback to static analysis on failures

### 4. Comprehensive Logging
- Logs all operations to `~/.claude/gograph-hook.log`
- Includes timestamps and detailed error messages
- Helps with debugging hook issues

## Security Considerations

- Runs with user permissions in current directory
- No sensitive data exposed in logs
- Uses structured JSON output to prevent injection
- Validates all input data before processing

## Testing

The `test-hook.ts` script provides comprehensive testing:

```bash
bun run test-hook
```

Tests include:
- Write operations on Go files
- Edit operations on Go files  
- MultiEdit operations with multiple changes
- Non-Go file handling (should skip)
- Error handling and edge cases

## Integration Flow

1. Claude Code triggers hook before Write/Edit/MultiEdit
2. Hook receives tool information via stdin
3. Hook analyzes Go file context
4. Returns approval with analysis feedback
5. Claude sees analysis and proceeds with operation
6. User benefits from contextual guidance

## Troubleshooting

### Hook Not Triggering
- Check matcher regex in settings.json
- Verify command path is absolute
- Ensure bun is accessible in PATH

### Analysis Not Showing
- Check exit code is 0
- Verify JSON output format
- Review logs for errors

### Permission Issues  
- Hook includes allowedTools configuration
- Add global permissions if needed
- Ensure gograph MCP server is running