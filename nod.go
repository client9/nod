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
	Tag      string
	Value    string
	Children []Node
	Line     int // 1-based line number of the tag
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
		tag      string
		value    string
		line     int
		children []*iNode
	}

	type frame struct {
		indent int
		node   *iNode
	}

	root := &iNode{}
	stack := []frame{{indent: -1, node: root}}
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))

		// Split on the first space to get the tag; take the rest of the line
		// verbatim (leading spaces trimmed) so interior whitespace in quoted
		// and backtick values is not collapsed.
		tag := trimmed
		rawValue := ""
		if i := strings.IndexByte(trimmed, ' '); i >= 0 {
			tag = trimmed[:i]
			rawValue = strings.TrimLeft(trimmed[i+1:], " ")
		}

		nodeLine := lineNum // capture before parseValue advances lineNum
		value := parseValue(rawValue, scanner, &lineNum)

		node := &iNode{tag: tag, value: value, line: nodeLine}

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
				Tag:      n.tag,
				Value:    n.value,
				Line:     n.line,
				Children: convert(n.children),
			}
		}
		return result
	}

	return convert(root.children), nil
}

// parseValue scans the raw value string using the tokenization state machine,
// consuming additional lines from scanner when a backtick block spans lines.
// All content is emitted as-is; no delimiters are stripped or escape sequences
// processed. See the String Tokenization Rules section in spec.md.
func parseValue(raw string, scanner *bufio.Scanner, lineNum *int) string {
	if raw == "" {
		return ""
	}
	var result strings.Builder
	cur := raw
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
					*lineNum++
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
	if hasNewline || ( hasQuote && !hasBacktick) {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// Write formats nodes as Nod and writes to w. Children are indented with two
// spaces per level. Values are written as-is; callers are responsible for
// encoding quoted strings and backtick blocks in Node.Value before calling.
func Write(w io.Writer, nodes []Node) error {
	bw := bufio.NewWriter(w)
	if err := writeNodes(bw, nodes, 0); err != nil {
		return err
	}
	return bw.Flush()
}

func writeNodes(w *bufio.Writer, nodes []Node, depth int) error {
	prefix := strings.Repeat(" ", depth*2)
	for _, n := range nodes {
		var err error
		if n.Value == "" {
			_, err = fmt.Fprintf(w, "%s%s\n", prefix, n.Tag)
		} else {
			_, err = fmt.Fprintf(w, "%s%s %s\n", prefix, n.Tag, n.Value)
		}
		if err != nil {
			return err
		}
		if err := writeNodes(w, n.Children, depth+1); err != nil {
			return err
		}
	}
	return nil
}
