#!/usr/bin/env bun

/**
 * test-hook.ts
 * Test script for gograph Claude Code hook
 */

import { spawn } from "child_process";
import { join } from "path";

// Test data for different scenarios matching Claude Code hook input format
const testCases = [
  {
    name: "✅ Valid Go file - should approve",
    input: {
      session_id: "test-123",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/test.go",
        content: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n"
      }
    },
    expectedDecision: "approve"
  },
  {
    name: "❌ Invalid Go file - missing package declaration",
    input: {
      session_id: "test-invalid-package",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/invalid.go",
        content: "func main() {\n\tfmt.Println(\"Missing package!\")\n}\n"
      }
    },
    expectedDecision: "block"
  },
  {
    name: "❌ Invalid Go syntax - using 'function' instead of 'func'",
    input: {
      session_id: "test-invalid-syntax",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/invalid_syntax.go",
        content: "package main\n\nfunction main() {\n\tconsole.log(\"Wrong syntax!\")\n}\n"
      }
    },
    expectedDecision: "block"
  },
  {
    name: "❌ Invalid Go concept - using 'class'",
    input: {
      session_id: "test-invalid-class",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/invalid_class.go",
        content: "package main\n\nclass MyClass {\n\tpublic function method() {}\n}\n"
      }
    },
    expectedDecision: "block"
  },
  {
    name: "⚠️ Valid Go with warnings - dash in filename",
    input: {
      session_id: "test-warning",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/test-file.go",
        content: "package main\n\nfunc main() {}\n"
      }
    },
    expectedDecision: "approve"
  },
  {
    name: "✅ Edit Go file - should approve",
    input: {
      session_id: "test-456",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Edit",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/existing.go",
        old_string: "old code",
        new_string: "new code"
      }
    },
    expectedDecision: "approve"
  },
  {
    name: "🚫 Non-Go file (should skip)",
    input: {
      session_id: "test-789",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/test.txt",
        content: "hello world"
      }
    },
    expectedDecision: "skip"
  },
  {
    name: "✅ MultiEdit Go file - should approve",
    input: {
      session_id: "test-multi",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "MultiEdit",
      tool_input: {
        file_path: "/Users/pedronauck/Dev/ai/gograph/multi.go",
        edits: [
          { old_string: "foo", new_string: "bar" },
          { old_string: "baz", new_string: "qux" }
        ]
      }
    },
    expectedDecision: "approve"
  },
  {
    name: "❌ Invalid file path",
    input: {
      session_id: "test-invalid-path",
      transcript_path: "~/.claude/projects/test/session.jsonl",
      tool_name: "Write",
      tool_input: {
        file_path: "", // Empty path
        content: "package main\n\nfunc main() {}\n"
      }
    },
    expectedDecision: "block"
  }
];

async function testHook(testCase: typeof testCases[0]): Promise<boolean> {
  console.log(`\n🧪 Testing: ${testCase.name}`);
  console.log("=" .repeat(50));
  
  return new Promise((resolve, reject) => {
    const hookPath = join(__dirname, "gograph-hook.ts");
    const child = spawn("bun", ["run", hookPath], {
      cwd: __dirname, // Run from scripts directory where this test is located
      stdio: ["pipe", "pipe", "pipe"]
    });
    
    let stdout = "";
    let stderr = "";
    
    child.stdout.on("data", (data) => {
      stdout += data.toString();
    });
    
    child.stderr.on("data", (data) => {
      stderr += data.toString();
    });
    
    child.on("close", (code) => {
      console.log(`Exit code: ${code}`);
      
      let testPassed = false;
      let actualDecision = "unknown";
      
      // Check exit code behavior
      if (testCase.expectedDecision === "skip" && code === 0) {
        // Non-Go files should exit cleanly without output
        testPassed = !stdout.trim();
        actualDecision = "skip";
      } else if (testCase.expectedDecision === "block" && code === 2) {
        // Blocked operations should exit with code 2
        testPassed = true;
        actualDecision = "block";
      } else if (testCase.expectedDecision === "approve" && code === 0) {
        // Approved operations should exit with code 0 and have JSON output
        testPassed = !!stdout.trim();
        actualDecision = "approve";
      }
      
      if (stdout) {
        console.log("📤 Output:");
        try {
          const output = JSON.parse(stdout);
          console.log(JSON.stringify(output, null, 2));
          
          // Validate decision in JSON output
          if (output.decision) {
            actualDecision = output.decision;
            testPassed = testPassed && (output.decision === testCase.expectedDecision);
          }
        } catch {
          console.log(stdout);
        }
      }
      
      if (stderr) {
        console.log("⚠️  Stderr:");
        console.log(stderr);
      }
      
      // Report test result
      const statusIcon = testPassed ? "✅" : "❌";
      console.log(`${statusIcon} Expected: ${testCase.expectedDecision}, Got: ${actualDecision}`);
      
      if (!testPassed) {
        console.log(`❌ TEST FAILED: Expected '${testCase.expectedDecision}' but got '${actualDecision}'`);
      }
      
      resolve(testPassed);
    });
    
    child.on("error", (error) => {
      console.error("❌ Process error:", error);
      reject(error);
    });
    
    // Send test input
    child.stdin.write(JSON.stringify(testCase.input));
    child.stdin.end();
  });
}

async function main(): Promise<void> {
  console.log("🚀 Starting gograph hook tests...");
  console.log(`Testing ${testCases.length} scenarios for validation behavior\n`);
  
  let passedTests = 0;
  let failedTests = 0;
  const failedTestNames: string[] = [];
  
  for (const testCase of testCases) {
    try {
      const testPassed = await testHook(testCase);
      if (testPassed) {
        passedTests++;
      } else {
        failedTests++;
        failedTestNames.push(testCase.name);
      }
    } catch (error) {
      console.error(`❌ Test failed for ${testCase.name}:`, error);
      failedTests++;
      failedTestNames.push(testCase.name);
    }
  }
  
  console.log("\n" + "=".repeat(60));
  console.log("📊 TEST SUMMARY");
  console.log("=".repeat(60));
  console.log(`✅ Passed: ${passedTests}/${testCases.length}`);
  console.log(`❌ Failed: ${failedTests}/${testCases.length}`);
  
  if (failedTests > 0) {
    console.log("\n❌ Failed tests:");
    failedTestNames.forEach(name => console.log(`  - ${name}`));
    console.log("\n🚨 Hook validation behavior needs attention!");
    process.exit(1);
  } else {
    console.log("\n🎉 All tests passed! Hook is working correctly.");
    console.log("✅ Validation and blocking behavior is functioning as expected.");
  }
}

if (import.meta.main) {
  main();
}