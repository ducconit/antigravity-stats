package analyzer

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type conversationMeta struct {
	Title         string
	Preview       string
	WorkspaceURIs string
	Project       string
	LastModified  time.Time
}

// loadSummariesFromSQLite loads titles and metadata from conversation_summaries.db.
func loadSummariesFromSQLite(dataDir string) map[string]conversationMeta {
	metaMap := make(map[string]conversationMeta)
	dbPath := filepath.Join(dataDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return metaMap
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return metaMap
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT c.conversation_id, c.title, c.preview, c.workspace_uris, c.last_modified_time, COALESCE(f.project, '')
		FROM conversation_summaries c
		LEFT JOIN (SELECT DISTINCT conversation_id, project FROM conversations_fts WHERE project != '') f
		ON c.conversation_id = f.conversation_id
	`)
	if err != nil {
		rows, err = db.Query("SELECT conversation_id, title, preview, workspace_uris, last_modified_time, '' FROM conversation_summaries")
		if err != nil {
			return metaMap
		}
	}
	defer rows.Close()

	for rows.Next() {
		var id, title, preview, workspaceURIs, lastModStr, proj string
		if err := rows.Scan(&id, &title, &preview, &workspaceURIs, &lastModStr, &proj); err == nil {
			normalized := strings.Replace(lastModStr, " ", "T", 1)
			t, _ := time.Parse(time.RFC3339, normalized)
			metaMap[id] = conversationMeta{
				Title:         title,
				Preview:       preview,
				WorkspaceURIs: workspaceURIs,
				Project:       proj,
				LastModified:  t,
			}
		}
	}
	return metaMap
}
