package claude

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type subagentTranscript struct {
	path        string
	description string
	agentType   string
}

type claudeTranscript struct {
	agents          map[string][]subagentTranscript
	agentTranscript bool
	ancestors       map[string]bool
}

func findSubagents(sessionPath string) map[string][]subagentTranscript {
	root := filepath.Join(strings.TrimSuffix(sessionPath, ".jsonl"), "subagents")
	agents := make(map[string][]subagentTranscript)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() || !strings.HasSuffix(path, ".meta.json") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var meta struct {
			ToolUseID   string `json:"toolUseId"`
			Description string `json:"description"`
			AgentType   string `json:"agentType"`
		}
		if json.Unmarshal(raw, &meta) != nil || meta.ToolUseID == "" {
			return nil
		}
		log := strings.TrimSuffix(path, ".meta.json") + ".jsonl"
		if info, err := os.Lstat(log); err == nil && info.Mode().IsRegular() {
			agents[meta.ToolUseID] = append(agents[meta.ToolUseID], subagentTranscript{log, meta.Description, meta.AgentType})
		}
		return nil
	})
	return agents
}

func (t claudeTranscript) appendSubagent(messages []Message, toolID string, ts time.Time, verbose bool) []Message {
	for _, agent := range t.agents[toolID] {
		if t.ancestors[agent.path] {
			continue
		}
		label := "Subagent"
		if agent.agentType != "" {
			label += " (" + agent.agentType + ")"
		}
		heading := label
		if agent.description != "" {
			heading += " · " + firstLine(agent.description)
		}
		messages = append(messages, Message{Role: "event", Text: heading, Timestamp: ts})
		if !verbose {
			continue
		}
		child := t
		child.agentTranscript = true
		if child.ancestors == nil {
			child.ancestors = make(map[string]bool)
		}
		child.ancestors[agent.path] = true
		turns, err := parseTranscript(agent.path, true, child.appendLine)
		delete(child.ancestors, agent.path)
		if err != nil {
			continue
		}
		for _, turn := range turns {
			if turn.Role != "agent" && turn.Role != "event" {
				turn = agentTurn(turn, label)
			}
			messages = append(messages, turn)
		}
	}
	return messages
}

func agentTurn(turn Message, label string) Message {
	switch turn.Role {
	case "user":
		label += " · User"
	case "assistant":
		label += " · Assistant"
	case "tool":
		label += " · Tool"
	case "tool_result":
		label += " · Result"
	}
	turn.Role = "agent"
	turn.Text = label + "\n" + turn.Text
	return turn
}
