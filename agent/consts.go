package agent

// checklistSystemPrompt is injected for parent agents to instruct the LLM about checklists.
const checklistSystemPrompt = `You must use the create_checklist tool as your very first action.
If the user's task solution is multi-step or multi-instruction and benefits from decomposition into subtasks, provide a list of items.
Each checklist item will be executed by a separate subagent with only the description and seed_context as input.`

// subagentSystemPrompt is injected for subagents.
const subagentSystemPrompt = "You are a focused subagent. Complete the following task thoroughly and respond with your final result. Do not create checklists."

const explanationPrompt = "Explain the following bash command in one short, succinct sentence. Do not leave out any important detail like flags, arguments, or side effects."

const compactPrompt = "You are a chat history compressor. Compress the following conversation into a concise summary that preserves all key information, decisions, and context. Return ONLY the compressed text, no explanations."
