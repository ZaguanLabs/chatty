package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	chattyErrors "github.com/ZaguanLabs/chatty/internal/errors"
)

const (
	defaultDirName  = ".local/share/chatty"
	defaultFileName = "chatty.db"
	timestampLayout = time.RFC3339

	// Security constants
	maxSessionNameLength = 200
	maxMessageLength     = 100000 // 100KB max message size
	maxRoleLength        = 50
	minRoleLength        = 1
)

// Store wraps access to the persistent conversation database.
type Store struct {
	db            *sql.DB
	preparedStmts map[string]*sql.Stmt
	preparedMutex sync.RWMutex
}

// Message represents a persisted chat message.
type Message struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

// SessionSummary describes a saved conversation.
type SessionSummary struct {
	ID           int64
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	MessageCount int
}

// Transcript bundles a session summary with its messages.
type Transcript struct {
	Summary  SessionSummary
	Messages []Message
}

// PaginationOptions holds pagination parameters for loading messages.
type PaginationOptions struct {
	Page     int // 1-based page number
	PageSize int // Number of messages per page
}

// Open initialises the storage layer, creating the database if necessary.
func Open(path string) (*Store, error) {
	return OpenWithPool(path, 1) // Pool size ignored
}

// OpenWithPool creates a store. maxConnections parameter is ignored in favor of safe single-connection usage.
func OpenWithPool(path string, maxConnections int) (*Store, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	// Use connection string parameters for timeout and WAL
	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_journal_mode=WAL", resolved)
	
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, chattyErrors.NewStorageError("open", fmt.Sprintf("failed to open sqlite database: %v", err), err)
	}

	// Force single connection to prevent locking issues
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Run verification pragmas
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, chattyErrors.NewStorageError("setup", fmt.Sprintf("failed to enable foreign keys: %v", err), err)
	}

	store := &Store{
		db: db,
	}

	if err := store.migrate(); err != nil {
		store.Close()
		return nil, err
	}

	if err := store.initializePreparedStatements(); err != nil {
		store.Close()
		return nil, err
	}

	return store, nil
}

// initializePreparedStatements sets up frequently used prepared statements.
func (s *Store) initializePreparedStatements() error {
	s.preparedStmts = make(map[string]*sql.Stmt)

	stmts := map[string]string{
		"createSession":        `INSERT INTO sessions(name) VALUES (?)`,
		"updateSessionName":    `UPDATE sessions SET name = ?, updated_at = (strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?`,
		"appendMessage":        `INSERT INTO messages(session_id, role, content) VALUES (?, ?, ?)`,
		"touchSession":         `UPDATE sessions SET updated_at = (strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?`,
		"listSessions":         `SELECT s.id, s.name, s.created_at, s.updated_at, COUNT(m.id) AS message_count FROM sessions s LEFT JOIN messages m ON m.session_id = s.id GROUP BY s.id ORDER BY s.updated_at DESC LIMIT ?`,
		"listSessionsNoLimit":  `SELECT s.id, s.name, s.created_at, s.updated_at, COUNT(m.id) AS message_count FROM sessions s LEFT JOIN messages m ON m.session_id = s.id GROUP BY s.id ORDER BY s.updated_at DESC`,
		"getSession":           `SELECT s.id, s.name, s.created_at, s.updated_at, COUNT(m.id) AS message_count FROM sessions s LEFT JOIN messages m ON m.session_id = s.id WHERE s.id = ? GROUP BY s.id`,
		"getMessages":          `SELECT role, content, created_at FROM messages WHERE session_id = ? ORDER BY id ASC`,
		"getMessagesPaginated": `SELECT role, content, created_at FROM messages WHERE session_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		"getMessageCount":      `SELECT COUNT(*) FROM messages WHERE session_id = ?`,
	}

	for name, query := range stmts {
		stmt, err := s.db.Prepare(query)
		if err != nil {
			return fmt.Errorf("prepare statement %s: %w", name, err)
		}
		s.preparedStmts[name] = stmt
	}

	return nil
}

// Close releases underlying database resources and prepared statements.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}

	var firstError error

	// Close prepared statements
	s.preparedMutex.Lock()
	for _, stmt := range s.preparedStmts {
		if err := stmt.Close(); err != nil && firstError == nil {
			firstError = err
		}
	}
	s.preparedStmts = nil
	s.preparedMutex.Unlock()

	// Close main database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil && firstError == nil {
			firstError = err
		}
	}

	return firstError
}

// AppendMessagesBatch appends multiple messages to the specified session in a single transaction.
func (s *Store) AppendMessagesBatch(ctx context.Context, sessionID int64, messages []Message) error {
	if s == nil {
		return chattyErrors.NewStorageError("batch", "store is nil", nil)
	}
	if sessionID <= 0 {
		return chattyErrors.NewValidationError("sessionID", "must be greater than 0", sessionID, nil)
	}
	if len(messages) == 0 {
		return nil // Nothing to do
	}

	// Use main connection directly
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed to begin transaction: %v", err), err)
	}
	defer tx.Rollback()

	// Prepare statements within transaction
	appendStmt, err := tx.PrepareContext(ctx, "INSERT INTO messages(session_id, role, content) VALUES (?, ?, ?)")
	if err != nil {
		return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed to prepare append statement: %v", err), err)
	}
	defer appendStmt.Close()

	touchStmt, err := tx.PrepareContext(ctx, "UPDATE sessions SET updated_at = (strftime('%Y-%m-%dT%H:%M:%SZ','now')) WHERE id = ?")
	if err != nil {
		return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed to prepare touch statement: %v", err), err)
	}
	defer touchStmt.Close()

	// Insert all messages
	for _, message := range messages {
		if strings.TrimSpace(message.Role) == "" {
			return chattyErrors.NewValidationError("message.role", "cannot be empty", message.Role, nil)
		}

		_, err := appendStmt.ExecContext(ctx, sessionID, message.Role, message.Content)
		if err != nil {
			return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed to insert message: %v", err), err)
		}
	}

	// Touch session to update timestamp
	if _, err := touchStmt.ExecContext(ctx, sessionID); err != nil {
		return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed to touch session: %v", err), err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed to commit transaction: %v", err), err)
	}

	return nil
}

// SaveMessagesWithRetry saves messages with automatic retry on failure
func (s *Store) SaveMessagesWithRetry(ctx context.Context, sessionID int64, messages []Message, maxRetries int) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := s.AppendMessagesBatch(ctx, sessionID, messages)
		if err == nil {
			return nil // Success
		}
		lastErr = err

		// Don't retry on validation errors
		if _, ok := err.(*chattyErrors.ValidationError); ok {
			return err
		}

		// Wait before retry (exponential backoff)
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
		}
	}
	return chattyErrors.NewStorageError("batch", fmt.Sprintf("failed after %d retries: %v", maxRetries, lastErr), lastErr)
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
            updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
        );`,
		`CREATE TABLE IF NOT EXISTS messages (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            session_id INTEGER NOT NULL,
            role TEXT NOT NULL,
            content TEXT NOT NULL,
            created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
            FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
        );`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("apply migration: %w", err)
		}
	}

	return nil
}

// getPreparedStmt safely retrieves a prepared statement.
func (s *Store) getPreparedStmt(name string) (*sql.Stmt, error) {
	s.preparedMutex.RLock()
	stmt := s.preparedStmts[name]
	s.preparedMutex.RUnlock()

	if stmt == nil {
		return nil, fmt.Errorf("prepared statement %s not found", name)
	}

	return stmt, nil
}

// CreateSession inserts a new conversation row and returns its identifier.
func (s *Store) CreateSession(ctx context.Context, name string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("storage not initialised")
	}

	title := strings.TrimSpace(name)

	// Enhanced input validation
	if title == "" {
		title = fmt.Sprintf("Session %s", time.Now().Format("2006-01-02 15:04"))
	} else {
		// Validate session name
		if err := validateSessionName(title); err != nil {
			return 0, chattyErrors.NewValidationError("name", err.Error(), title, err)
		}

		// Sanitize the name
		title = sanitizeString(title, maxSessionNameLength)
	}

	stmt, err := s.getPreparedStmt("createSession")
	if err != nil {
		return 0, err
	}

	res, err := stmt.ExecContext(ctx, title)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("resolve session id: %w", err)
	}

	return id, nil
}

// UpdateSessionName updates the stored name for a session.
func (s *Store) UpdateSessionName(ctx context.Context, id int64, name string) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialised")
	}
	if id <= 0 {
		return errors.New("invalid session id")
	}

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("session name cannot be empty")
	}

	stmt, err := s.getPreparedStmt("updateSessionName")
	if err != nil {
		return err
	}

	if _, err := stmt.ExecContext(ctx, trimmed, id); err != nil {
		return fmt.Errorf("update session name: %w", err)
	}

	return nil
}

// AppendMessage appends a message to the specified session.
func (s *Store) AppendMessage(ctx context.Context, sessionID int64, message Message) error {
	if s == nil || s.db == nil {
		return errors.New("storage not initialised")
	}

	// Enhanced session ID validation
	if sessionID <= 0 {
		return errors.New("invalid session id")
	}
	if sessionID > 9223372036854775807 { // Max int64
		return errors.New("session id too large")
	}

	// Enhanced message validation
	if strings.TrimSpace(message.Role) == "" {
		return errors.New("message role cannot be empty")
	}

	// Validate role
	if err := validateMessageRole(message.Role); err != nil {
		return chattyErrors.NewValidationError("role", err.Error(), message.Role, err)
	}

	// Validate content
	if err := validateMessageContent(message.Content); err != nil {
		return chattyErrors.NewValidationError("content", err.Error(), message.Content, err)
	}

	// Sanitize inputs
	// sanitizedRole := sanitizeString(message.Role, maxRoleLength)
	// sanitizedContent := sanitizeString(message.Content, maxMessageLength)

	// Use prepared statement for appending message
	stmt, err := s.getPreparedStmt("appendMessage")
	if err != nil {
		return err
	}

	if _, err := stmt.ExecContext(ctx, sessionID, message.Role, message.Content); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// Touch session to update updated_at timestamp
	touchStmt, err := s.getPreparedStmt("touchSession")
	if err != nil {
		return err
	}

	if _, err := touchStmt.ExecContext(ctx, sessionID); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}

	return nil
}

// ListSessions returns stored conversations ordered by most recent activity.
func (s *Store) ListSessions(ctx context.Context, limit int) ([]SessionSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("storage not initialised")
	}

	if limit > 0 {
		stmt, err := s.getPreparedStmt("listSessions")
		if err != nil {
			return nil, err
		}
		rows, err := stmt.QueryContext(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("list sessions: %w", err)
		}
		defer rows.Close()
		return s.scanSessionSummaries(rows)
	} else {
		stmt, err := s.getPreparedStmt("listSessionsNoLimit")
		if err != nil {
			return nil, err
		}
		rows, err := stmt.QueryContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("list sessions: %w", err)
		}
		defer rows.Close()
		return s.scanSessionSummaries(rows)
	}
}

// scanSessionSummaries scans session summary rows into structs.
func (s *Store) scanSessionSummaries(rows *sql.Rows) ([]SessionSummary, error) {
	summaries := make([]SessionSummary, 0, 8)
	for rows.Next() {
		var summary SessionSummary
		var created, updated string
		if scanErr := rows.Scan(&summary.ID, &summary.Name, &created, &updated, &summary.MessageCount); scanErr != nil {
			return nil, fmt.Errorf("scan session summary: %w", scanErr)
		}

		var parseErr error
		summary.CreatedAt, parseErr = parseTimestamp(created)
		if parseErr != nil {
			return nil, parseErr
		}
		summary.UpdatedAt, parseErr = parseTimestamp(updated)
		if parseErr != nil {
			return nil, parseErr
		}
		summaries = append(summaries, summary)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate session summaries: %w", rowsErr)
	}

	return summaries, nil
}

// LoadSession fetches the session metadata and full transcript for the given identifier.
func (s *Store) LoadSession(ctx context.Context, id int64) (*Transcript, error) {
	return s.LoadSessionWithPagination(ctx, id, nil)
}

// LoadSessionWithPagination fetches the session metadata and messages with optional pagination.
func (s *Store) LoadSessionWithPagination(ctx context.Context, id int64, pagination *PaginationOptions) (*Transcript, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("storage not initialised")
	}
	if id <= 0 {
		return nil, errors.New("invalid session id")
	}

	var summary SessionSummary
	var created, updated string
	stmt, err := s.getPreparedStmt("getSession")
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRowContext(ctx, id)
	if err := row.Scan(&summary.ID, &summary.Name, &created, &updated, &summary.MessageCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session %d not found", id)
		}
		return nil, fmt.Errorf("select session: %w", err)
	}

	var parseErr error
	summary.CreatedAt, parseErr = parseTimestamp(created)
	if parseErr != nil {
		return nil, parseErr
	}
	summary.UpdatedAt, parseErr = parseTimestamp(updated)
	if parseErr != nil {
		return nil, parseErr
	}

	// Determine pagination settings
	pageSize := 50 // Default page size

	if pagination != nil {
		if pagination.PageSize > 0 {
			pageSize = pagination.PageSize
		}
	}

	// If pagination is requested and there are many messages, use pagination
	if pagination != nil || summary.MessageCount > 100 {
		// Get message count using prepared statement
		countStmt, err := s.getPreparedStmt("getMessageCount")
		if err != nil {
			return nil, err
		}

		var totalCount int
		var countErr error
		countErr = countStmt.QueryRowContext(ctx, id).Scan(&totalCount)
		if countErr != nil {
			return nil, fmt.Errorf("get message count: %w", countErr)
		}
		summary.MessageCount = totalCount

		// Calculate offset (for backward pagination, we get messages from the end)
		var actualOffset int
		if pagination != nil && pagination.Page > 0 {
			actualOffset = (pagination.Page - 1) * pageSize
		} else {
			// Get most recent messages (paginated from the end)
			actualOffset = totalCount - pageSize
			if actualOffset < 0 {
				actualOffset = 0
			}
		}

		// Use paginated query
		paginatedStmt, err := s.getPreparedStmt("getMessagesPaginated")
		if err != nil {
			return nil, err
		}
		rows, err := paginatedStmt.QueryContext(ctx, id, pageSize, actualOffset)
		if err != nil {
			return nil, fmt.Errorf("load messages paginated: %w", err)
		}
		defer rows.Close()

		messages := make([]Message, 0, pageSize)
		for rows.Next() {
			var msg Message
			var createdAt string
			if err := rows.Scan(&msg.Role, &msg.Content, &createdAt); err != nil {
				return nil, fmt.Errorf("scan message: %w", err)
			}
			msg.CreatedAt, err = parseTimestamp(createdAt)
			if err != nil {
				return nil, err
			}
			messages = append(messages, msg)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate messages: %w", err)
		}

		// Reverse messages to show chronological order
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		return &Transcript{Summary: summary, Messages: messages}, nil
	}

	// Load all messages using prepared statement (for smaller conversations)
	msgStmt, err := s.getPreparedStmt("getMessages")
	if err != nil {
		return nil, err
	}
	rows, err := msgStmt.QueryContext(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0, summary.MessageCount)
	for rows.Next() {
		var msg Message
		var createdAt string
		if err := rows.Scan(&msg.Role, &msg.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msg.CreatedAt, err = parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return &Transcript{Summary: summary, Messages: messages}, nil
}

func resolvePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		trimmed = filepath.Join(home, defaultDirName, defaultFileName)
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create storage directory: %w", err)
	}

	return absPath, nil
}

func parseTimestamp(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(timestampLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", value, err)
	}
	return t, nil
}

// validateSessionName validates session name for security
func validateSessionName(name string) error {
	trimmed := strings.TrimSpace(name)

	if trimmed == "" {
		return errors.New("session name cannot be empty")
	}

	if len(trimmed) > maxSessionNameLength {
		return fmt.Errorf("session name too long (max %d characters)", maxSessionNameLength)
	}

	// Basic character validation - only allow safe characters
	for _, char := range trimmed {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
		     (char >= '0' && char <= '9') || char == ' ' || char == '-' ||
		     char == '_' || char == '.' || char == '(' || char == ')') {
			return errors.New("session name contains invalid characters")
		}
	}

	return nil
}

// sanitizeString performs basic sanitization on strings
func sanitizeString(input string, maxLength int) string {
	if input == "" {
		return ""
	}

	// Trim whitespace
	trimmed := strings.TrimSpace(input)

	// Limit length
	if len(trimmed) > maxLength {
		trimmed = trimmed[:maxLength]
	}

	// Remove null bytes
	trimmed = strings.ReplaceAll(trimmed, "\x00", "")

	return trimmed
}

// validateMessageRole validates message role for security
func validateMessageRole(role string) error {
	trimmed := strings.TrimSpace(role)

	if trimmed == "" {
		return errors.New("message role cannot be empty")
	}

	if len(trimmed) > maxRoleLength {
		return fmt.Errorf("message role too long (max %d characters)", maxRoleLength)
	}

	// Check against valid roles
	validRoles := []string{"user", "assistant", "system"}
	isValid := false
	for _, validRole := range validRoles {
		if trimmed == validRole {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid message role '%s' (must be one of: user, assistant, system)", trimmed)
	}

	return nil
}

// validateMessageContent validates message content for security
func validateMessageContent(content string) error {
	trimmed := strings.TrimSpace(content)

	if trimmed == "" {
		return errors.New("message content cannot be empty")
	}

	if len(trimmed) > maxMessageLength {
		return fmt.Errorf("message content too long (max %d characters)", maxMessageLength)
	}

	// Check for obvious XSS attempts
	xssPattern := regexp.MustCompile(`(?i)(<script|<iframe|javascript:|onerror=|onload=|onclick=)`)
	if xssPattern.MatchString(trimmed) {
		return errors.New("message content appears to contain XSS attempt")
	}

	// Check for control characters (except common ones like newline, tab)
	for _, char := range trimmed {
		if char < 32 && char != '\n' && char != '\r' && char != '\t' {
			return errors.New("message content contains invalid control characters")
		}
	}

	return nil
}
