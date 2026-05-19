package nod

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Node is a single parsed node from a Nod document.
type Node struct {
	Line     string // trimmed line content (tag + value); may contain \n for multiline backtick blocks
	Children []Node
}

// Parse reads a Nod-format document and returns the top-level nodes.
func Parse(r io.Reader) ([]Node, error) {
	// iNode is an internal node used during parsing. Children are []*iNode
	// rather than []iNode so that pointers to nodes remain stable when a
	// parent's children slice is appended to and its underlying array is
	// reallocated. The indent stack holds *iNode pointers; if children were
	// stored by value, a reallocation would move them and leave the stack
	// holding dangling pointers into the old array. After parsing is complete
	// the tree is converted to the public []Node type.
	type iNode struct {
		line     string
		children []*iNode
	}

	type frame struct {
		indent int
		node   *iNode
	}

	root := &iNode{}
	stack := []frame{{indent: -1, node: root}}
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			// Blank line creates an empty node (Line == "") as a sibling of the
			// most recently processed node. We use the current stack top's indent
			// as the effective indent so normal dedenting fires, but the blank
			// node is not pushed — it cannot be a parent.
			effectiveIndent := stack[len(stack)-1].indent
			for len(stack) > 1 && stack[len(stack)-1].indent >= effectiveIndent {
				stack = stack[:len(stack)-1]
			}
			stack[len(stack)-1].node.children = append(
				stack[len(stack)-1].node.children, &iNode{})
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		content := scanLine(trimmed, scanner)
		node := &iNode{line: content}

		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1].node
		parent.children = append(parent.children, node)
		stack = append(stack, frame{indent: indent, node: node})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var convert func([]*iNode) []Node
	convert = func(nodes []*iNode) []Node {
		if len(nodes) == 0 {
			return nil
		}
		result := make([]Node, len(nodes))
		for i, n := range nodes {
			result[i] = Node{
				Line:     n.line,
				Children: convert(n.children),
			}
		}
		return result
	}

	return convert(root.children), nil
}

// scanLine scans the full trimmed line content using the tokenization state
// machine, consuming additional lines from scanner when a backtick block spans
// lines. All content is emitted as-is. See the String Tokenization Rules
// section in spec.md.
func scanLine(content string, scanner *bufio.Scanner) string {
	// Fast path: no quotes or backticks means no continuation lines possible.
	if !strings.ContainsAny(content, "\"`") {
		return content
	}
	var result strings.Builder
	cur := content
	pos := 0
	for pos < len(cur) {
		ch := cur[pos]
		switch ch {
		case '"':
			// Quoted string: emit everything including delimiters;
			// \ skips the next character so \" doesn't end the string.
			result.WriteByte('"')
			pos++
			for pos < len(cur) {
				c := cur[pos]
				if c == '\\' && pos+1 < len(cur) {
					result.WriteByte(c)
					result.WriteByte(cur[pos+1])
					pos += 2
				} else if c == '"' {
					result.WriteByte('"')
					pos++
					break
				} else {
					result.WriteByte(c)
					pos++
				}
			}
		case '`':
			// Backtick block: emit everything including delimiters;
			// may span multiple lines, no escape processing.
			result.WriteByte('`')
			pos++
			closed := false
			for !closed {
				for pos < len(cur) {
					c := cur[pos]
					if c == '`' {
						result.WriteByte('`')
						pos++
						closed = true
						break
					}
					result.WriteByte(c)
					pos++
				}
				if !closed {
					if !scanner.Scan() {
						return result.String()
					}
					result.WriteByte('\n')
					cur = scanner.Text()
					pos = 0
				}
			}
		default:
			result.WriteByte(ch)
			pos++
		}
	}
	return result.String()
}

// TrimIndent removes the common leading-space prefix from every line in s.
// Blank and whitespace-only lines are excluded when measuring the common
// prefix but are preserved as-is in the output.
func TrimIndent(s string) string {
	lines := strings.Split(s, "\n")
	min := -1
	for _, line := range lines {
		stripped := strings.TrimLeft(line, " ")
		if stripped == "" {
			continue
		}
		if n := len(line) - len(stripped); min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return s
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		if len(line) >= min {
			out[i] = line[min:]
		} else {
			out[i] = ""
		}
	}
	return strings.Join(out, "\n")
}

// AddIndent prepends indent to every non-empty line in s.
func AddIndent(s, indent string) string {
	if indent == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		if line != "" {
			out[i] = indent + line
		}
	}
	return strings.Join(out, "\n")
}

// NewNode creates a Node whose Line is set from args via Quote and Join,
// equivalent to calling SetArgs on a zero Node.
func NewNode(args ...string) Node {
	var n Node
	n.SetArgs(args)
	return n
}

// Head returns the first whitespace-delimited token of Line. Returns an empty
// string if Line is empty.
func (n Node) Head() string {
	i := strings.IndexByte(n.Line, ' ')
	if i < 0 {
		return n.Line
	}
	return n.Line[:i]
}

// Args parses Line into a slice of strings using shell-like tokenization.
// Tokens are separated by whitespace. Double-quoted strings are decoded with
// strconv.Unquote (Go string escape rules); on a malformed quoted string the
// raw token is returned unchanged. Backtick-quoted strings have their
// delimiters stripped and their content returned as-is (no escape processing).
func (n Node) Args() []string {
	var result []string
	s := n.Line
	for {
		// Skip whitespace between tokens.
		i := 0
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		s = s[i:]
		if s == "" {
			break
		}

		var token string
		switch s[0] {
		case '"':
			end := 1
			for end < len(s) {
				if s[end] == '\\' && end+1 < len(s) {
					end += 2
				} else if s[end] == '"' {
					end++
					break
				} else {
					end++
				}
			}
			raw := s[:end]
			s = s[end:]
			if unquoted, err := strconv.Unquote(raw); err == nil {
				token = unquoted
			} else {
				token = raw
			}
		case '`':
			end := 1
			for end < len(s) && s[end] != '`' {
				end++
			}
			if end < len(s) {
				token = s[1:end] // content between delimiters
				s = s[end+1:]
			} else {
				token = s[1:end]
				s = ""
			}
		default:
			end := 0
			for end < len(s) && s[end] != ' ' && s[end] != '\t' && s[end] != '\n' {
				end++
			}
			token = s[:end]
			s = s[end:]
		}
		result = append(result, token)
	}
	return result
}

// SetArgs encodes args into n.Line by quoting each argument with Quote and
// joining with spaces.
func (n *Node) SetArgs(args []string) {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = Quote(a)
	}
	n.Line = strings.Join(quoted, " ")
}

// Quote encodes s for use as a Nod value. If s contains no whitespace,
// double-quotes, or backticks, it is returned unchanged. If s contains a
// double-quote but no backtick, it is wrapped in backticks. Otherwise
// strconv.Quote is used.
func Quote(s string) string {
	hasSpace := strings.ContainsAny(s, " \t\n\r")
	hasQuote := strings.ContainsRune(s, '"')
	hasNewline := strings.ContainsRune(s, '\n')
	hasBacktick := strings.ContainsRune(s, '`')

	if !hasSpace && !hasQuote && !hasBacktick {
		return s
	}
	if hasNewline || (hasQuote && !hasBacktick) {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// Write formats nodes as Nod and writes to w. indent is prepended once per
// nesting level; a typical value is "  " (two spaces). Node.Line is written
// as-is; callers are responsible for encoding quoted strings and backtick
// blocks before calling.
func Write(w io.Writer, nodes []Node, indent string) error {
	bw := bufio.NewWriter(w)
	if err := writeNodes(bw, nodes, indent, 0); err != nil {
		return err
	}
	return bw.Flush()
}

func writeNodes(w *bufio.Writer, nodes []Node, indent string, depth int) error {
	prefix := strings.Repeat(indent, depth)
	for _, n := range nodes {
		if _, err := fmt.Fprintf(w, "%s%s\n", prefix, n.Line); err != nil {
			return err
		}
		if err := writeNodes(w, n.Children, indent, depth+1); err != nil {
			return err
		}
	}
	return nil
}
