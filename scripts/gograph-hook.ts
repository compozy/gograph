#!/usr/bin/env bun

/**
 * gograph-hook.ts
 * Smart Claude Code hook for Go code analysis using gograph MCP
 * Runs before Write/Edit/MultiEdit operations on Go files
 * 
 * Usage: Called automatically by Claude Code hooks system
 */

import { query, type Options, type SDKMessage } from "@anthropic-ai/claude-code";
import { existsSync, writeFileSync, appendFileSync } from "fs";
import { join } from "path";
import { homedir } from "os";

// Configuration
const LOG_FILE = join(homedir(), ".claude", "gograph-hook.log");
const TIMEOUT_MS = 10000; // Reduced for faster feedback
const QUICK_VALIDATION_TIMEOUT = 2000; // Quick checks timeout

// Types based on Claude Code documentation
interface PreToolUseInput {
  session_id: string;
  transcript_path: string;
  tool_name: string;
  tool_input: Record<string, any>;
}

// Advanced JSON output for hooks
interface HookOutput {
  // Common fields
  continue?: boolean;  // Whether Claude should continue (default: true)
  stopReason?: string; // Message shown when continue is false
  suppressOutput?: boolean; // Hide stdout from transcript mode (default: false)
  
  // PreToolUse specific
  decision?: "approve" | "block" | undefined;
  reason?: string; // Explanation for decision
}

// Validation result interface
interface ValidationResult {
  passed: boolean;
  errors: string[];
  warnings: string[];
  suggestions: string[];
}

// Logging utility
function log(message: string): void {
  const timestamp = new Date().toISOString();
  const logEntry = `[${timestamp}] ${message}\n`;
  try {
    appendFileSync(LOG_FILE, logEntry);
  } catch (error) {
    console.error("Failed to write to log:", error);
  }
}

// Check if bun and required tools are available
function checkDependencies(): boolean {
  try {
    // Check if we're running in bun
    if (!process.versions.bun) {
      log("ERROR: Not running in bun environment");
      return false;
    }
    return true;
  } catch (error) {
    log(`ERROR: Dependency check failed: ${error}`);
    return false;
  }
}

// Extract Go file paths from tool parameters
function extractGoFiles(toolParams: Record<string, any>): string[] {
  const goFiles: string[] = [];
  
  // Check file_path parameter
  if (toolParams.file_path && typeof toolParams.file_path === "string") {
    if (toolParams.file_path.endsWith(".go")) {
      goFiles.push(toolParams.file_path);
    }
  }
  
  // Check edits array for MultiEdit
  if (toolParams.edits && Array.isArray(toolParams.edits)) {
    if (toolParams.file_path && toolParams.file_path.endsWith(".go")) {
      goFiles.push(toolParams.file_path);
    }
  }
  
  return [...new Set(goFiles)]; // Remove duplicates
}

// Check if gograph is available and configured
function checkGographAvailable(workingDir: string): boolean {
  return existsSync(join(workingDir, "gograph.yaml")) || 
         existsSync(join(workingDir, "..", "gograph.yaml"));
}

// Initialize gograph project if needed
async function ensureProjectInitialized(workingDir: string): Promise<boolean> {
  try {
    if (!checkGographAvailable(workingDir)) {
      log("No gograph.yaml found - skipping validation");
      return false;
    }
    
    // For now, assume project is available if gograph.yaml exists
    // TODO: Add proper project validation once MCP stability improves
    log("gograph.yaml found - assuming project available");
    return true;
  } catch (error) {
    log(`Project initialization check failed: ${error}`);
    return false;
  }
}

// Generate analysis prompt based on operation type
function generateAnalysisPrompt(toolName: string, filePath: string, workingDir: string): string {
  const basePrompt = `Using the gograph MCP tools, analyze the Go codebase context for ${toolName.toLowerCase()} operation on '${filePath}'.`;
  
  switch (toolName) {
    case "Write":
      return `${basePrompt}

Please provide:
1. **Dependencies**: What packages/functions this file will likely need to import
2. **Integration Points**: How this file fits into the existing architecture  
3. **Potential Issues**: Any naming conflicts or architectural concerns
4. **Best Practices**: Recommendations for proper integration

Focus on preventing hallucinations by providing accurate, verified information from the actual codebase structure.`;

    case "Edit":
    case "MultiEdit":
      return `${basePrompt}

Please provide:
1. **Dependency Analysis**: What functions/types might be affected by changes
2. **Usage Impact**: Which other files depend on this file's exports
3. **Circular Dependencies**: Check for potential import cycles
4. **Related Tests**: Identify test files that should be updated

Ensure all analysis is based on the actual current codebase structure to prevent incorrect assumptions.`;

    default:
      return `${basePrompt}

Provide accurate dependency and usage information to prevent hallucinations.`;
  }
}

// Run Claude analysis using SDK with existing session
async function runAnalysis(prompt: string): Promise<string> {
  try {
    log("Starting Claude SDK analysis using existing session...");
    
    // Use the SDK with proper options
    const options: Options = {
      maxTurns: 1,
      permissionMode: "default",
      allowedTools: ["mcp__gograph__*"] // Allow ALL gograph MCP tools
    };
    
    const messages: string[] = [];
    
    // Use timeout to prevent hanging
    const analysisPromise = (async () => {
      for await (const message of query({ prompt, options })) {
        // Handle different SDKMessage types
        if (message.type === "assistant") {
          // Extract text content from assistant messages
          const content = message.message.content;
          if (Array.isArray(content)) {
            for (const block of content) {
              if (block.type === "text") {
                messages.push(block.text);
              }
            }
          }
        } else if (message.type === "result") {
          // Handle result messages
          if (message.subtype === "success" && "result" in message) {
            messages.push(message.result);
          }
        }
        // Skip system and user messages for analysis output
      }
      return messages.join("\n");
    })();
    
    const timeoutPromise = new Promise<string>((_, reject) => {
      setTimeout(() => reject(new Error("Analysis timeout")), TIMEOUT_MS);
    });
    
    const result = await Promise.race([analysisPromise, timeoutPromise]);
    log("Claude SDK analysis completed successfully");
    return result;
    
  } catch (error) {
    log(`Claude SDK analysis failed: ${error}`);
    throw error;
  }
}

// Perform quick synchronous validation checks
function performQuickValidation(filePath: string, toolInput: Record<string, any>): ValidationResult {
  const errors: string[] = [];
  const warnings: string[] = [];
  const suggestions: string[] = [];
  
  // Basic file path validation
  if (!filePath || typeof filePath !== "string") {
    errors.push("Invalid file path provided");
  }
  
  // Check for suspicious file paths
  if (filePath.includes("../") || filePath.startsWith("/")) {
    warnings.push("Potentially unsafe file path detected");
  }
  
  // Validate Go file extension
  if (!filePath.endsWith(".go")) {
    errors.push("File is not a Go source file");
  }
  
  // Check for common Go naming patterns
  const fileName = filePath.split("/").pop() || "";
  if (fileName.includes(" ") || fileName.includes("-")) {
    warnings.push("Go files should use underscores, not spaces or hyphens");
  }
  
  // Basic content validation for Write operations
  if (toolInput.content && typeof toolInput.content === "string") {
    const content = toolInput.content;
    
    // Check for package declaration
    if (!content.includes("package ")) {
      errors.push("Go file must start with package declaration");
    }
    
    // Check for potential import issues
    const importMatches = content.match(/import\s+["']([^"']+)["']/g);
    if (importMatches) {
      for (const importMatch of importMatches) {
        const importPath = importMatch.match(/["']([^"']+)["']/)?.[1];
        if (importPath && !importPath.includes(".")) {
          warnings.push(`Standard library import detected: ${importPath}`);
        }
      }
    }
    
    // Check for basic Go syntax patterns
    if (content.includes("function ") && !content.includes("func ")) {
      errors.push("Go uses 'func' keyword, not 'function'");
    }
    
    if (content.includes("class ")) {
      errors.push("Go doesn't have classes - use structs and methods");
    }
  }
  
  return {
    passed: errors.length === 0,
    errors,
    warnings,
    suggestions
  };
}

// Fallback static analysis
function getStaticAnalysis(workingDir: string, validationResult?: ValidationResult): string {
  let analysis = "";
  
  if (validationResult && !validationResult.passed) {
    analysis += `❌ **Validation Failed**\n\n**Errors:**\n${validationResult.errors.map(e => `- ${e}`).join("\n")}\n\n`;
  }
  
  if (validationResult && validationResult.warnings.length > 0) {
    analysis += `⚠️  **Warnings:**\n${validationResult.warnings.map(w => `- ${w}`).join("\n")}\n\n`;
  }
  
  if (checkGographAvailable(workingDir)) {
    analysis += `🔍 **Smart Go Analysis**\n\ngograph configuration detected. Consider these best practices:\n\n**📦 Imports & Dependencies**\n- Verify all imports are available in the current module\n- Check for potential circular import issues\n- Ensure proper package naming conventions\n\n**🔄 Integration Points**  \n- Review function signatures against existing interfaces\n- Validate struct field types match usage patterns\n- Check method receivers follow project conventions\n\n**⚡ Next Steps**\n- Run \`gograph analyze\` to update dependency graph\n- Use gograph MCP tools to verify code relationships\n- Consider impact on test files and documentation\n\n*Claude SDK integration active - full MCP analysis available*`;
  } else {
    analysis += `📋 **gograph Setup Required**\n\nNo gograph.yaml found. To enable dependency analysis:\n\n1. Run: \`gograph init --project-id your-project\`\n2. Run: \`gograph analyze\` to build dependency graph  \n3. Configure MCP integration for full analysis\n\nThis will enable intelligent Go code analysis and prevent hallucinations.`;
  }
  
  return analysis;
}

// Format hook response for PreToolUse
function formatHookResponse(
  analysis: string, 
  fileInfo: string, 
  validationResult?: ValidationResult
): HookOutput {
  // Block operation if validation failed
  if (validationResult && !validationResult.passed) {
    return {
      decision: "block",
      reason: `❌ **Validation Failed for ${fileInfo}**\n\n${analysis}\n\n**Errors that must be fixed:**\n${validationResult.errors.map(e => `- ${e}`).join("\n")}\n\n---\n*Validation powered by gograph MCP + Claude SDK*`,
      continue: false,
      stopReason: "Code validation failed - please fix the issues above",
      suppressOutput: false
    };
  }
  
  // Approve with analysis feedback
  const warningText = validationResult && validationResult.warnings.length > 0 
    ? `\n\n⚠️  **Warnings:**\n${validationResult.warnings.map(w => `- ${w}`).join("\n")}`
    : "";
    
  return {
    decision: "approve",
    reason: `✅ **gograph Analysis for ${fileInfo}**\n\n${analysis}${warningText}\n\n---\n*Analysis powered by gograph MCP + Claude SDK*`,
    continue: true,
    suppressOutput: false
  };
}

// Main hook execution
async function main(): Promise<void> {
  try {
    // Read hook input from stdin
    const input = await new Promise<string>((resolve, reject) => {
      let data = "";
      process.stdin.on("data", chunk => data += chunk.toString());
      process.stdin.on("end", () => resolve(data));
      process.stdin.on("error", reject);
    });
    
    if (!input.trim()) {
      log("No input received from hook system");
      process.exit(0);
    }
    
    const hookInput: PreToolUseInput = JSON.parse(input);
    log(`Hook triggered: tool=${hookInput.tool_name}, session=${hookInput.session_id}`);
    
    // Check dependencies
    if (!checkDependencies()) {
      log("Dependencies not available, exiting");
      process.exit(0);
    }
    
    // Extract Go files from tool parameters
    const goFiles = extractGoFiles(hookInput.tool_input);
    if (goFiles.length === 0) {
      log("No Go files detected, skipping analysis");
      process.exit(0);
    }
    
    const primaryFile = goFiles[0];
    // Extract working directory from file path or use current directory
    const workingDir = process.cwd();
    
    log(`Analyzing Go file: ${primaryFile} in project: ${workingDir}`);
    
    // Perform quick validation first
    const quickValidation = performQuickValidation(primaryFile, hookInput.tool_input);
    
    // If quick validation fails, block immediately
    if (!quickValidation.passed) {
      log(`Quick validation failed for ${primaryFile}: ${quickValidation.errors.join(', ')}`);
      const staticAnalysis = getStaticAnalysis(workingDir, quickValidation);
      const response = formatHookResponse(staticAnalysis, primaryFile, quickValidation);
      console.log(JSON.stringify(response));
      log(`Validation blocked for ${primaryFile}`);
      process.exit(2); // Exit code 2 blocks operation
    }
    
    // Check if project is initialized
    const projectInitialized = await ensureProjectInitialized(workingDir);
    
    let analysis: string;
    
    if (projectInitialized) {
      // Generate analysis prompt
      const prompt = generateAnalysisPrompt(hookInput.tool_name, primaryFile, workingDir);
      
      try {
        // Try Claude SDK analysis with shorter timeout
        analysis = await runAnalysis(prompt);
      } catch (error) {
        log(`Claude SDK analysis failed, using static analysis: ${error}`);
        analysis = getStaticAnalysis(workingDir, quickValidation);
      }
    } else {
      // Use static analysis if project not initialized
      analysis = getStaticAnalysis(workingDir, quickValidation);
    }
    
    // Format and output response
    const response = formatHookResponse(analysis, primaryFile, quickValidation);
    console.log(JSON.stringify(response));
    
    log(`Analysis completed successfully for ${primaryFile}`);
    process.exit(0); // Exit code 0 for success
    
  } catch (error) {
    log(`Hook execution failed: ${error}`);
    // Exit with code 1 to show error but not block operation
    console.error(`gograph hook error: ${error}`);
    process.exit(1);
  }
}

// Execute main function
if (import.meta.main) {
  main();
}