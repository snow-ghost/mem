import type { Plugin } from "@opencode-ai/plugin"

/**
 * mem — persistent memory plugin for OpenCode
 *
 * Automatically:
 * - Extracts significant events when a session goes idle (TUI mode)
 *   or when session status changes to "completed" (run mode)
 * - Injects memory context into the system prompt at session start
 *
 * Requirements:
 * - `mem` binary in PATH (https://github.com/snow-ghost/mem)
 * - `mem init` run once in the project directory
 */
export const MemPlugin: Plugin = async ({ $, directory }) => {
  const memPath = `${directory}/.memory`

  // Check if mem is available
  const check = await $`which mem`.quiet().nothrow()
  if (check.exitCode !== 0) {
    console.warn("[mem] mem binary not found in PATH, plugin disabled")
    return {}
  }

  // Check if memory store is initialized
  const initCheck =
    await $`test -f ${memPath}/episodes.jsonl`.quiet().nothrow()
  if (initCheck.exitCode !== 0) {
    console.warn(
      `[mem] memory store not initialized in ${directory}, run: mem init`,
    )
    return {}
  }

  let extracted = false

  const doExtract = async () => {
    if (extracted) return
    extracted = true
    const result =
      await $`MEM_BACKEND=opencode mem extract --path ${memPath}`
        .quiet()
        .nothrow()
    if (result.exitCode === 0) {
      const output = result.stdout.toString().trim()
      if (output && !output.includes("No significant events")) {
        console.log(`[mem] ${output.split("\n")[0]}`)
      }
    }
  }

  return {
    // Inject memory context into the system prompt
    "experimental.chat.system.transform": async (input) => {
      const result =
        await $`mem inject --path ${memPath}`.quiet().nothrow()
      if (result.exitCode === 0 && result.stdout.toString().trim()) {
        input.system = result.stdout.toString() + "\n\n" + input.system
      }
      return input
    },

    // Extract when session goes idle (interactive TUI mode)
    "session.idle": async () => {
      await doExtract()
    },

    // Also catch session status changes (covers "run" mode completion)
    event: async ({ event }) => {
      if (
        event.type === "session.status" &&
        "properties" in event &&
        (event as any).properties?.status === "completed"
      ) {
        await doExtract()
      }
    },
  }
}
