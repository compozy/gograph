# gograph Claude Code Hook Integration

This guide shows how to manually integrate the gograph hook with your Claude Code settings.

## Prerequisites

- [bun](https://bun.sh) installed
- Claude Code CLI installed and authenticated
- gograph project with `gograph.yaml` configuration

## Setup Steps

### 1. Install Dependencies

```bash
# From project root
bun install
```

### 2. Update Claude Code Settings

Add the hook configuration to your `~/.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "cd /path/to/your/gograph && bun run scripts/gograph-hook.ts"
          }
        ]
      }
    ]
  }
}
```

**Important**: Replace `/path/to/your/gograph` with the actual absolute path to your gograph project root directory.

### 3. Optional: Configure Global MCP Tool Permissions

The hook includes `allowedTools` configuration to permit gograph MCP tools automatically. However, you can also add global permissions to your `~/.claude/settings.json` if preferred:

```json
{
  "permissions": {
    "allow": [
      "mcp__gograph__analyze_project",
      "mcp__gograph__get_package_structure", 
      "mcp__gograph__query_dependencies",
      "mcp__gograph__get_function_info",
      "mcp__gograph__natural_language_query"
    ]
  }
}
```

Note: The hook automatically includes these permissions via `allowedTools` configuration, so this step is optional.

### 4. Complete Settings Example

Here's a minimal `~/.claude/settings.json` example:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "cd /Users/username/projects/gograph && bun run scripts/gograph-hook.ts"
          }
        ]
      }
    ]
  }
}
```

The hook handles tool permissions automatically via the SDK's `allowedTools` configuration.

## Testing the Integration

### 1. Test the Hook Directly

```bash
cd scripts/
bun run test-hook.ts
```

### 2. Test with Claude Code

1. Open Claude Code in a Go project with gograph configuration
2. Try editing or creating a `.go` file
3. The hook should trigger and provide analysis before the operation
4. Check logs at `~/.claude/gograph-hook.log` if needed

## How It Works

1. **Trigger**: Hook activates before `Write`, `Edit`, or `MultiEdit` operations on `.go` files
2. **Analysis**: Uses Claude Code SDK with your existing session to analyze via gograph MCP
3. **Output**: Provides dependency analysis, architectural recommendations, and integration guidance
4. **Fallback**: If MCP analysis fails, provides static analysis based on gograph configuration

## Configuration Options

### Hook Matchers

You can customize which operations trigger the hook:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "cd /path/to/your/gograph && bun run scripts/gograph-hook.ts"
          }
        ]
      }
    ]
  }
}
```

You can also use more specific regex patterns:
- `"Write"` - Only Write operations
- `"Edit|MultiEdit"` - Only Edit operations
- `".*"` - All tools
- `"Notebook.*"` - All notebook operations

### Working Directory

The hook automatically detects your project's working directory from the Claude Code session.

### Permissions

Without the permissions configuration, Claude Code will prompt you to approve each MCP tool use. Adding the permissions list prevents these prompts for a smoother experience.

## Troubleshooting

### Hook Not Triggering
- Check that the path in your settings.json is correct and absolute
- Ensure bun is installed and accessible
- Verify the hook file is executable: `chmod +x scripts/gograph-hook.ts`

### Permission Errors
- Add the gograph MCP tool permissions to your settings.json
- Ensure your gograph MCP server is running and connected

### Analysis Failures
- Check that you have a `gograph.yaml` file in your project
- Verify gograph MCP tools are working: test with Claude Code directly
- Check logs at `~/.claude/gograph-hook.log`

## Security Considerations

- The hook uses your existing Claude Code authentication session
- No API keys or additional authentication required
- Only processes Go files in projects with gograph configuration
- Logs activity to `~/.claude/gograph-hook.log` for transparency

## Uninstalling

To remove the hook integration:

1. Remove the hook entry from your `~/.claude/settings.json`
2. Optionally remove the gograph MCP permissions
3. Delete the scripts directory if no longer needed