# gograph Claude Code Hook

Smart Claude Code hook for Go code analysis using gograph MCP integration.

## Overview

This hook integrates with Claude Code to provide intelligent Go code analysis before file operations. It uses the Claude Code SDK and gograph MCP tools to:

- Analyze dependencies and imports
- Check for potential issues and conflicts  
- Provide architectural recommendations
- Prevent hallucinations with actual codebase context

## Files

- `gograph-hook.ts` - Main hook implementation using Claude Code SDK
- `test-hook.ts` - Test suite for validating hook functionality  
- `INTEGRATION.md` - Manual integration guide
- `README.md` - This documentation file
- `../package.json` - Node.js dependencies (Claude Code SDK) in project root
- `../tsconfig.json` - TypeScript configuration for the project

## Setup

1. Install dependencies from project root:
   ```bash
   bun install
   ```
   
2. Follow the integration guide: [INTEGRATION.md](./INTEGRATION.md)

You must manually configure Claude Code settings following the integration guide - no automatic setup is provided to respect user settings.

## Configuration

The hook is automatically configured to:
- Trigger on `Write`, `Edit`, and `MultiEdit` operations
- Only process Go files (`.go` extension)
- Use existing Claude Code authentication session
- Grant permissions for gograph MCP tools to avoid blocking

## Permissions

The installation script automatically adds these permissions to prevent prompting:
- `mcp__gograph__analyze_project`
- `mcp__gograph__get_package_structure` 
- `mcp__gograph__query_dependencies`
- `mcp__gograph__get_function_info`
- `mcp__gograph__natural_language_query`
- `mcp__gograph__trace_call_chain`
- `mcp__gograph__detect_circular_deps`
- `mcp__gograph__find_implementations`
- `mcp__gograph__verify_code_exists`

## Testing

Run the test suite to validate functionality:
```bash
cd scripts/
bun run test-hook.ts
```

## Logs

Hook activity is logged to: `~/.claude/gograph-hook.log`

## Requirements

- bun runtime
- Claude Code with active session
- gograph project with `gograph.yaml` configuration
- gograph MCP server running and connected