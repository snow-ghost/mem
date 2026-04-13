package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
	"github.com/snow-ghost/mem/internal/embeddings"
	"github.com/snow-ghost/mem/internal/kg"
	"github.com/snow-ghost/mem/internal/layers"
	"github.com/snow-ghost/mem/internal/palace"
	"github.com/snow-ghost/mem/internal/search"
)

func NewServer(d *db.DB, cfg config.Config) *mcp.Server {
	s := mcp.NewServer(
		&mcp.Implementation{Name: "mem", Version: "0.2.0"},
		nil,
	)

	s.AddTool(&mcp.Tool{
		Name:        "mem_search",
		Description: "Search memories in the palace. Mode: bm25 (default), vector, or hybrid (requires MEM_EMBEDDINGS_*).",
		InputSchema: jsonSchema(map[string]any{
			"query": map[string]any{"type": "string", "description": "search query"},
			"wing":  map[string]any{"type": "string", "description": "filter by wing"},
			"room":  map[string]any{"type": "string", "description": "filter by room"},
			"mode":  map[string]any{"type": "string", "description": "bm25 | vector | hybrid (default bm25)"},
		}, []string{"query"}),
	}, searchHandler(d, cfg))

	s.AddTool(&mcp.Tool{
		Name:        "mem_add_drawer",
		Description: "Store content in the palace.",
		InputSchema: jsonSchema(map[string]any{
			"content": map[string]any{"type": "string", "description": "text to store"},
			"wing":    map[string]any{"type": "string", "description": "wing name"},
			"room":    map[string]any{"type": "string", "description": "room name"},
		}, []string{"content", "wing"}),
	}, addDrawerHandler(d, cfg))

	s.AddTool(&mcp.Tool{
		Name:        "mem_status",
		Description: "Palace overview.",
		InputSchema: jsonSchema(map[string]any{}, nil),
	}, statusHandler(d, cfg))

	s.AddTool(&mcp.Tool{
		Name:        "mem_wake_up",
		Description: "Compact context for AI session.",
		InputSchema: jsonSchema(map[string]any{
			"wing": map[string]any{"type": "string", "description": "filter by wing"},
		}, nil),
	}, wakeUpHandler(d, cfg))

	s.AddTool(&mcp.Tool{
		Name:        "mem_kg_query",
		Description: "Query knowledge graph entity.",
		InputSchema: jsonSchema(map[string]any{
			"entity": map[string]any{"type": "string", "description": "entity name"},
			"as_of":  map[string]any{"type": "string", "description": "date filter"},
		}, []string{"entity"}),
	}, kgQueryHandler(d))

	s.AddTool(&mcp.Tool{
		Name:        "mem_kg_add",
		Description: "Add fact to knowledge graph.",
		InputSchema: jsonSchema(map[string]any{
			"subject":   map[string]any{"type": "string"},
			"predicate": map[string]any{"type": "string"},
			"object":    map[string]any{"type": "string"},
			"from":      map[string]any{"type": "string"},
		}, []string{"subject", "predicate", "object"}),
	}, kgAddHandler(d))

	s.AddTool(&mcp.Tool{
		Name:        "mem_list_wings",
		Description: "List all wings.",
		InputSchema: jsonSchema(map[string]any{}, nil),
	}, listWingsHandler(d))

	s.AddTool(&mcp.Tool{
		Name:        "mem_list_rooms",
		Description: "List rooms in a wing.",
		InputSchema: jsonSchema(map[string]any{
			"wing": map[string]any{"type": "string"},
		}, []string{"wing"}),
	}, listRoomsHandler(d))

	return s
}

func jsonSchema(props map[string]any, required []string) json.RawMessage {
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	data, _ := json.Marshal(schema)
	return data
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func getArgs(req *mcp.CallToolRequest) map[string]string {
	result := make(map[string]string)
	if req.Params.Arguments == nil {
		return result
	}
	var raw map[string]any
	json.Unmarshal(req.Params.Arguments, &raw)
	for k, v := range raw {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

func searchHandler(d *db.DB, cfg config.Config) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := getArgs(req)
		var wingID, roomID int64
		if a["wing"] != "" {
			if w, err := palace.GetWing(d, a["wing"]); err == nil {
				wingID = w.ID
			}
		}
		if a["room"] != "" && wingID > 0 {
			if r, err := palace.GetRoom(d, a["room"], wingID); err == nil {
				roomID = r.ID
			}
		}

		mode := a["mode"]
		var results []search.SearchResult
		var err error
		switch mode {
		case "vector", "hybrid":
			if !cfg.EmbeddingsEnabled() {
				return textResult("Error: vector/hybrid mode requires MEM_EMBEDDINGS_URL and MEM_EMBEDDINGS_MODEL"), nil
			}
			client := embeddings.NewClient(cfg)
			qvec, embErr := client.Embed(a["query"])
			if embErr != nil {
				return textResult(fmt.Sprintf("Error embedding query: %v", embErr)), nil
			}
			if mode == "vector" {
				results, err = search.SearchVector(d, qvec, wingID, roomID, 5)
			} else {
				results, err = search.SearchHybrid(d, a["query"], qvec, wingID, roomID, 5)
			}
		default:
			results, err = search.Search(d, a["query"], wingID, roomID, 5)
		}
		if err != nil {
			return textResult(fmt.Sprintf("Error: %v", err)), nil
		}
		if len(results) == 0 {
			return textResult("No results"), nil
		}
		var out string
		for i, r := range results {
			out += fmt.Sprintf("[%d] %s/%s (%.3f)\n%s\n%s\n\n", i+1, r.WingName, r.RoomName, r.Score, filepath.Base(r.SourceFile), r.Content)
		}
		return textResult(out), nil
	}
}

func addDrawerHandler(d *db.DB, cfg config.Config) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := getArgs(req)
		roomName := a["room"]
		if roomName == "" {
			roomName = "general"
		}
		wing, _ := palace.CreateWing(d, a["wing"], "general", "")
		room, _ := palace.CreateRoom(d, roomName, wing.ID)
		drawer, _ := palace.AddDrawer(d, a["content"], wing.ID, room.ID, "facts", "", "manual")
		if drawer == nil {
			return textResult("Duplicate"), nil
		}
		search.IndexDrawer(d, drawer.ID, a["content"])

		// Auto-embed if configured. Failure is non-fatal — the drawer is
		// still stored and BM25-indexed; embedding can be retried via
		// `mem reindex`.
		if cfg.EmbeddingsEnabled() {
			client := embeddings.NewClient(cfg)
			if vec, err := client.Embed(a["content"]); err == nil {
				search.IndexEmbedding(d, drawer.ID, embeddings.Encode(vec))
			}
		}

		return textResult(fmt.Sprintf("Stored in %s/%s (#%d)", a["wing"], roomName, drawer.ID)), nil
	}
}

func statusHandler(d *db.DB, cfg config.Config) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s, _ := palace.GetStatus(d, cfg.PalacePath)
		return textResult(fmt.Sprintf("Wings: %d, Rooms: %d, Drawers: %d", s.Wings, s.Rooms, s.Drawers)), nil
	}
}

func wakeUpHandler(d *db.DB, cfg config.Config) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := getArgs(req)
		return textResult(layers.WakeUp(d, cfg, a["wing"])), nil
	}
}

func kgQueryHandler(d *db.DB) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := getArgs(req)
		results, _ := kg.QueryEntity(d, a["entity"], a["as_of"], "both")
		if len(results) == 0 {
			return textResult("No facts found"), nil
		}
		var out string
		for _, r := range results {
			out += fmt.Sprintf("%s → %s → %s (%s)\n", r.SubjName, r.Predicate, r.ObjName, r.ValidFrom)
		}
		return textResult(out), nil
	}
}

func kgAddHandler(d *db.DB) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := getArgs(req)
		id, err := kg.AddTriple(d, a["subject"], a["predicate"], a["object"], a["from"], "")
		if err != nil {
			return textResult(fmt.Sprintf("Error: %v", err)), nil
		}
		return textResult(fmt.Sprintf("Added: %s → %s → %s (%s)", a["subject"], a["predicate"], a["object"], id[:12])), nil
	}
}

func listWingsHandler(d *db.DB) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		wings, _ := palace.ListWings(d)
		var out string
		for _, w := range wings {
			out += fmt.Sprintf("- %s (%s)\n", w.Name, w.Type)
		}
		if out == "" {
			return textResult("No wings"), nil
		}
		return textResult(out), nil
	}
}

func listRoomsHandler(d *db.DB) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a := getArgs(req)
		w, err := palace.GetWing(d, a["wing"])
		if err != nil {
			return textResult("Wing not found"), nil
		}
		rooms, _ := palace.ListRooms(d, w.ID)
		var out string
		for _, r := range rooms {
			out += fmt.Sprintf("- %s\n", r.Name)
		}
		if out == "" {
			return textResult("No rooms"), nil
		}
		return textResult(out), nil
	}
}
