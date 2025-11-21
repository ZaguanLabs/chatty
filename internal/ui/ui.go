package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Colors provides ANSI color constants for modern terminal rendering
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Faint     = "\033[2m"
	Normal    = "\033[22m"

	// Modern color palette
	DeepBlue    = "\033[38;5;24m"   // Deep blue for user messages
	DeepGreen   = "\033[38;5;28m"   // Deep green for assistant messages
	Gray        = "\033[38;5;245m"  // Light gray for system text
	DarkGray    = "\033[38;5;238m"  // Dark gray for code blocks
	Orange      = "\033[38;5;208m"  // Orange for commands
	Magenta     = "\033[38;5;201m"  // Magenta for highlights
	Cyan        = "\033[38;5;51m"   // Cyan for code
	Yellow      = "\033[38;5;226m"  // Yellow for warnings
	BrightWhite = "\033[38;5;231m"  // Bright white for text

	// Background colors
	BGBlue      = "\033[48;5;17m"   // Very dark blue background
	BGGreen     = "\033[48;5;22m"   // Very dark green background
	BGGray      = "\033[48;5;235m"  // Dark gray background
	BGUser      = "\033[48;5;17m"   // User message background
	BGAssistant = "\033[48;5;22m"   // Assistant message background
	BGSystem    = "\033[48;5;235m"  // System message background

	// Border colors
	BorderBlue   = "\033[38;5;24m"  // Deep blue for user borders
	BorderGreen  = "\033[38;5;28m"  // Deep green for assistant borders
	BorderGray   = "\033[38;5;245m" // Medium gray for borders
)

// GetUserAvatar returns the avatar emoji for user messages
func GetUserAvatar() string {
	return "👤"
}

// GetAssistantAvatar returns the avatar emoji for assistant messages
func GetAssistantAvatar() string {
	return "🤖"
}

// GetUserName returns the display name for user messages
func GetUserName() string {
	return "You"
}

// GetAssistantName returns the display name for assistant messages
func GetAssistantName() string {
	return "Assistant"
}

// FormatTimestamp formats a time with modern styling
func FormatTimestamp(t time.Time) string {
	return t.Format("3:04 PM")
}

// FormatShortTimestamp formats time in a compact format
func FormatShortTimestamp(t time.Time) string {
	return t.Format("15:04")
}

// CreateSeparator creates a decorative separator line
func CreateSeparator(width int, style string) string {
	switch style {
	case "thick":
		return strings.Repeat("═", width)
	case "thin":
		return strings.Repeat("─", width)
	case "dots":
		return strings.Repeat("•", width)
	case "asterisks":
		return strings.Repeat("✦", width)
	case "spaces":
		return strings.Repeat(" ", width)
	case "dashed":
		return strings.Repeat("┄", width)
	default:
		return strings.Repeat("─", width)
	}
}

// CreateBox creates a bordered text box
func CreateBox(content, title string, width int) string {
	if width < 10 {
		width = 40
	}

	if len(content) > width-4 {
		content = content[:width-7] + "..."
	}

	padding := width - len(content) - 4
	if padding < 0 {
		padding = 0
	}

	var box strings.Builder
	box.WriteString("┌" + CreateSeparator(width-2, "thin") + "┐\n")

	if title != "" {
		box.WriteString("│ " + title)
		if len(title) < width-4 {
			box.WriteString(strings.Repeat(" ", width-4-len(title)))
		}
		box.WriteString(" │\n")
		box.WriteString("├" + CreateSeparator(width-2, "thin") + "┤\n")
	}

	box.WriteString("│" + strings.Repeat(" ", 1) + content)
	box.WriteString(strings.Repeat(" ", padding))
	box.WriteString(" │\n")
	box.WriteString("└" + CreateSeparator(width-2, "thin") + "┘")

	return box.String()
}

// CreateMessageFooter creates a styled message footer with bottom border
func CreateMessageFooter(msgType string, width int) string {
	var borderColor string

	switch msgType {
	case "user":
		borderColor = BorderBlue
	case "assistant":
		borderColor = BorderGreen
	default:
		borderColor = BorderGray
	}

	if width <= 0 {
		width = 50
	}

	return borderColor + "└" + strings.Repeat("─", width-2) + "┘" + Reset
}

// CreateMessageHeader creates a styled message header with avatar and timestamp
func CreateMessageHeader(msgType string, timestamp time.Time) string {
	var avatar, name string
	var color, bgColor, borderColor string

	switch msgType {
	case "user":
		avatar = GetUserAvatar()
		name = GetUserName()
		color = BrightWhite
		bgColor = BGUser
		borderColor = BorderBlue
	case "assistant":
		avatar = GetAssistantAvatar()
		name = GetAssistantName()
		color = BrightWhite
		bgColor = BGAssistant
		borderColor = BorderGreen
	default:
		avatar = "💬"
		name = "Message"
		color = BrightWhite
		bgColor = BGSystem
		borderColor = BorderGray
	}

	timestampStr := FormatTimestamp(timestamp)
	headerText := fmt.Sprintf("%s %s │ %s", avatar, name, timestampStr)

	// Create a bordered header with background
	width := len(headerText) + 4
	if width < 20 {
		width = 20
	}

	// Top border
	topBorder := borderColor + "┌─ " + name + " " + strings.Repeat("─", width-len(name)-5) + "┐" + Reset

	// Header content with background
	headerLine := bgColor + borderColor + "│ " + Reset + color + headerText +
		strings.Repeat(" ", width-len(headerText)-3) + bgColor + borderColor + " │" + Reset

	// Bottom border
	bottomBorder := borderColor + "├" + strings.Repeat("─", width-2) + "┤" + Reset

	return topBorder + "\n" + headerLine + "\n" + bottomBorder
}

// CreateStatusMessage creates a styled status message
func CreateStatusMessage(emoji, message, statusType string) string {
	var color string
	
	switch statusType {
	case "success":
		color = "\033[38;5;82m" // Bright green
	case "error":
		color = "\033[38;5;196m" // Bright red
	case "warning":
		color = Yellow
	case "info":
		color = Cyan
	default:
		color = Gray
	}
	
	return fmt.Sprintf("%s%s %s%s", color, emoji, message, Reset)
}

// GetLanguageEmoji returns an emoji for common programming languages
func GetLanguageEmoji(lang string) string {
	lang = strings.ToLower(lang)
	
	switch {
	case strings.Contains(lang, "python"):
		return "🐍"
	case strings.Contains(lang, "javascript"), strings.Contains(lang, "js"):
		return "🟨"
	case strings.Contains(lang, "typescript"), strings.Contains(lang, "ts"):
		return "🔷"
	case strings.Contains(lang, "go"):
		return "🔵"
	case strings.Contains(lang, "rust"):
		return "🦀"
	case strings.Contains(lang, "java"):
		return "☕"
	case strings.Contains(lang, "c++"), strings.Contains(lang, "cpp"):
		return "⚡"
	case strings.Contains(lang, "html"):
		return "🌐"
	case strings.Contains(lang, "css"):
		return "🎨"
	case strings.Contains(lang, "json"):
		return "📋"
	case strings.Contains(lang, "yaml"), strings.Contains(lang, "yml"):
		return "⚙️"
	case strings.Contains(lang, "bash"), strings.Contains(lang, "shell"):
		return "💻"
	default:
		return "📄"
	}
}

// CreateCodeBlock creates a styled code block with language detection
func CreateCodeBlock(code, language string) string {
	emoji := GetLanguageEmoji(language)
	
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s┌─ %s %s ─┐%s\n", DarkGray, emoji, language, Reset))
	sb.WriteString(fmt.Sprintf("%s│%s\n", DarkGray, Reset))
	
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if i < len(lines)-1 || line != "" {
			sb.WriteString(fmt.Sprintf("%s│%s %s%s\n", DarkGray, Reset, line, Reset))
		}
	}
	
	sb.WriteString(fmt.Sprintf("%s└%s %s ───────┘%s\n", DarkGray, strings.Repeat("─", len(language)+len(" ")+len(emoji)), CreateSeparator(len(language)+4, "spaces"), Reset))
	
	return sb.String()
}

// GetLoadingFrame returns a frame for the loading animation
func GetLoadingFrame(index int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[index%len(frames)]
}

// CreateLoadingMessage creates a styled loading message
func CreateLoadingMessage(icon, message string, frameIndex int) string {
	frame := GetLoadingFrame(frameIndex)
	return fmt.Sprintf("%s%s %s %s%s", Cyan, frame, icon, message, Reset)
}

// GetTerminalWidth detects terminal width with fallback
func GetTerminalWidth() int {
	// This is a simplified version - in a real implementation,
	// you'd use a proper terminal detection library
	return 80
}

// GetTerminalWidthWithSession gets terminal width from session, with fallback
func GetTerminalWidthWithSession(sessionWidth int) int {
	if sessionWidth > 0 {
		return sessionWidth
	}
	return GetTerminalWidth()
}

// CreateSeparatorWithWidth creates a decorative separator line with specific width
func CreateSeparatorWithWidth(width int, style string) string {
	if width <= 0 {
		width = 50 // Default fallback
	}

	switch style {
	case "thick":
		return strings.Repeat("═", width)
	case "thin":
		return strings.Repeat("─", width)
	case "dots":
		return strings.Repeat("•", width)
	case "asterisks":
		return strings.Repeat("✦", width)
	case "spaces":
		return strings.Repeat(" ", width)
	default:
		return strings.Repeat("─", width)
	}
}

// CreateMessageHeaderWithWidth creates a styled message header with specific width
func CreateMessageHeaderWithWidth(msgType string, timestamp time.Time, terminalWidth int) string {
	var avatar, name string
	var color string

	switch msgType {
	case "user":
		avatar = GetUserAvatar()
		name = GetUserName()
		color = DeepBlue
	case "assistant":
		avatar = GetAssistantAvatar()
		name = GetAssistantName()
		color = DeepGreen
	default:
		avatar = "💬"
		name = "Message"
		color = Gray
	}

	timestampStr := FormatTimestamp(timestamp)

	return fmt.Sprintf("%s%s %s%s │ %s%s",
		color, avatar, Bold+name, Normal, Gray, timestampStr)
}

// CreateCodeBlockWithWidth creates a styled code block with specific width
func CreateCodeBlockWithWidth(code, language string, terminalWidth int) string {
	emoji := GetLanguageEmoji(language)

	// Calculate reasonable width for code blocks
	if terminalWidth <= 0 {
		terminalWidth = 80
	}

	// Leave margin for borders and padding
	codeWidth := terminalWidth - 6

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s┌─ %s %s ─┐%s\n", DarkGray, emoji, language, Reset))
	sb.WriteString(fmt.Sprintf("%s│%s\n", DarkGray, Reset))

	lines := strings.Split(code, "\n")
	for _, line := range lines {
		if line != "" {
			// Truncate long lines to fit terminal
			displayLine := line
			if len(displayLine) > codeWidth-2 {
				displayLine = displayLine[:codeWidth-5] + "..."
			}
			sb.WriteString(fmt.Sprintf("%s│%s %s%s\n", DarkGray, Reset, displayLine, Reset))
		}
	}

	sb.WriteString(fmt.Sprintf("%s└", DarkGray))
	padding := len(language) + len(emoji) + 3
	sb.WriteString(strings.Repeat("─", padding))
	sb.WriteString("┘")
	sb.WriteString(fmt.Sprintf("%s\n", Reset))

	return sb.String()
}

// TruncateWithIndicator truncates text with a show-more indicator
func TruncateWithIndicator(text, indicator string, maxWidth int) string {
	if len(text) <= maxWidth {
		return text
	}
	
	truncated := text[:maxWidth-len(indicator)-3]
	return truncated + "..." + indicator
}

// WrapText wraps text to specified width
func WrapText(text string, width int) []string {
	lines := strings.Split(text, "\n")
	var result []string
	
	for _, line := range lines {
		if len(line) <= width {
			result = append(result, line)
			continue
		}
		
		// Simple word wrapping
		words := strings.Fields(line)
		currentLine := ""
		
		for _, word := range words {
			if len(currentLine)+len(word)+1 <= width {
				if currentLine != "" {
					currentLine += " " + word
				} else {
					currentLine = word
				}
			} else {
				if currentLine != "" {
					result = append(result, currentLine)
					currentLine = word
				} else {
					result = append(result, word)
				}
			}
		}
		
		if currentLine != "" {
			result = append(result, currentLine)
		}
	}
	
	return result
}

// CreateProgressBar creates a visual progress bar
func CreateProgressBar(current, total int, width int) string {
	if total == 0 {
		return strings.Repeat("░", width)
	}
	
	progress := float64(current) / float64(total)
	filled := int(progress * float64(width))
	
	if filled > width {
		filled = width
	}
	
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// FormatFileSize formats file sizes in human readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	value := float64(bytes) / float64(div)
	return fmt.Sprintf("%.1f %cB", value, "KMGTPE"[exp])
}

// CreateBulletPoint creates a styled bullet point
func CreateBulletPoint(text, bulletType string) string {
	var bullet string
	switch bulletType {
	case "arrow":
		bullet = "▸"
	case "circle":
		bullet = "●"
	case "diamond":
		bullet = "◆"
	case "square":
		bullet = "■"
	case "dot":
		bullet = "•"
	default:
		bullet = "•"
	}
	
	return fmt.Sprintf("%s %s %s", Cyan+bullet+Reset, text, Reset)
}