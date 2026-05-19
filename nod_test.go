package nod

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func write(t *testing.T, nodes []Node) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, nodes); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	return buf.String()
}

func parse(t *testing.T, src string) []Node {
	t.Helper()
	nodes, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	return nodes
}

// ---- basic structure ----

func TestTagOnly(t *testing.T) {
	nodes := parse(t, "name\n")
	if len(nodes) != 1 {
		t.Fatalf("want 1 node, got %d", len(nodes))
	}
	if nodes[0].Line != "name" {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestTagWithValue(t *testing.T) {
	nodes := parse(t, "name Edward Thomas Miller\n")
	if nodes[0].Line != "name Edward Thomas Miller" {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestMultipleTopLevel(t *testing.T) {
	nodes := parse(t, "a\nb\nc\n")
	if len(nodes) != 3 {
		t.Fatalf("want 3 nodes, got %d", len(nodes))
	}
	for i, want := range []string{"a", "b", "c"} {
		if nodes[i].Line != want {
			t.Errorf("nodes[%d].Line = %q, want %q", i, nodes[i].Line, want)
		}
	}
}

func TestEmpty(t *testing.T) {
	nodes := parse(t, "")
	if len(nodes) != 0 {
		t.Fatalf("want 0 nodes, got %d", len(nodes))
	}
}

// ---- blank lines ----

func TestBlankLinesIgnored(t *testing.T) {
	nodes := parse(t, "\n\na\n\nb\n\n")
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
}

// ---- comments ----

func TestCommentBecomesNode(t *testing.T) {
	nodes := parse(t, "# a comment\na\n")
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Line != "# a comment" {
		t.Errorf("got %q", nodes[0].Line)
	}
	if nodes[1].Line != "a" {
		t.Errorf("got %q", nodes[1].Line)
	}
}

func TestCommentAttachesToCurrentContext(t *testing.T) {
	// A comment attaches to whatever node is on top of the indent stack at the
	// time it appears, without modifying the stack. Subsequent regular nodes
	// continue as if the comment wasn't there.
	src := "birth\n  date 1923\n  # comment\n  place Toledo\n"
	nodes := parse(t, src)
	if len(nodes) != 1 {
		t.Fatalf("want 1 top-level node, got %d", len(nodes))
	}
	birth := nodes[0]
	// birth.Children = [date, comment (child of date), place]
	// comment is a child of date (the stack-top at the time)
	if len(birth.Children) != 2 {
		t.Fatalf("birth: want 2 children, got %d: %v", len(birth.Children), birth.Children)
	}
	date := birth.Children[0]
	if date.Line != "date 1923" {
		t.Errorf("date.Line = %q", date.Line)
	}
	if len(date.Children) != 1 || date.Children[0].Line != "# comment" {
		t.Errorf("date.Children = %v", date.Children)
	}
	if birth.Children[1].Line != "place Toledo" {
		t.Errorf("place.Line = %q", birth.Children[1].Line)
	}
}

func TestCommentAtTopLevelBetweenNodes(t *testing.T) {
	src := "a\n# comment\nb\n"
	nodes := parse(t, src)
	// comment attaches to a (the stack-top), a and b are both top-level
	if len(nodes) != 2 {
		t.Fatalf("want 2 top-level nodes, got %d", len(nodes))
	}
	if nodes[0].Line != "a" || nodes[1].Line != "b" {
		t.Errorf("got %q, %q", nodes[0].Line, nodes[1].Line)
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].Line != "# comment" {
		t.Errorf("a.Children = %v", nodes[0].Children)
	}
}

func TestInlineHashIsLiteral(t *testing.T) {
	nodes := parse(t, "note see also: church records # not a comment\n")
	want := "note see also: church records # not a comment"
	if nodes[0].Line != want {
		t.Errorf("got %q, want %q", nodes[0].Line, want)
	}
}

// ---- nesting ----

func TestChildren(t *testing.T) {
	nodes := parse(t, "birth\n  date 1923-04-17\n  place Toledo\n")
	if len(nodes) != 1 {
		t.Fatalf("want 1 top-level node, got %d", len(nodes))
	}
	birth := nodes[0]
	if birth.Line != "birth" {
		t.Fatalf("line = %q", birth.Line)
	}
	if len(birth.Children) != 2 {
		t.Fatalf("want 2 children, got %d", len(birth.Children))
	}
	if birth.Children[0].Line != "date 1923-04-17" {
		t.Errorf("child[0]: %q", birth.Children[0].Line)
	}
	if birth.Children[1].Line != "place Toledo" {
		t.Errorf("child[1]: %q", birth.Children[1].Line)
	}
}

func TestDeepNesting(t *testing.T) {
	src := "person Alice\n  birth\n    date 1960-02-03\n    place Chicago\n"
	nodes := parse(t, src)
	if len(nodes) != 1 {
		t.Fatalf("want 1 top-level, got %d", len(nodes))
	}
	birth := nodes[0].Children[0]
	if birth.Line != "birth" || len(birth.Children) != 2 {
		t.Errorf("unexpected birth node: %+v", birth)
	}
}

func TestNodeWithValueAndChildren(t *testing.T) {
	src := "footnote edward-birth-cert\n  text Ohio birth certificate.\n"
	nodes := parse(t, src)
	fn := nodes[0]
	if fn.Line != "footnote edward-birth-cert" {
		t.Errorf("line = %q", fn.Line)
	}
	if len(fn.Children) != 1 || fn.Children[0].Line != "text Ohio birth certificate." {
		t.Errorf("children = %+v", fn.Children)
	}
}

func TestSiblingsAfterChildren(t *testing.T) {
	src := "a\n  b\nc\n"
	nodes := parse(t, src)
	if len(nodes) != 2 {
		t.Fatalf("want 2 top-level nodes, got %d", len(nodes))
	}
	if nodes[1].Line != "c" {
		t.Errorf("nodes[1].Line = %q", nodes[1].Line)
	}
}

func TestNonUniformIndent(t *testing.T) {
	src := "a\n   b\n      c\n   d\n"
	nodes := parse(t, src)
	if len(nodes) != 1 {
		t.Fatalf("want 1 top-level, got %d", len(nodes))
	}
	a := nodes[0]
	if len(a.Children) != 2 {
		t.Fatalf("want 2 children of a, got %d", len(a.Children))
	}
	b := a.Children[0]
	if len(b.Children) != 1 || b.Children[0].Line != "c" {
		t.Errorf("b.Children = %+v", b.Children)
	}
	if a.Children[1].Line != "d" {
		t.Errorf("a.Children[1].Line = %q", a.Children[1].Line)
	}
}

// ---- implicit lists ----

func TestRepeatedTags(t *testing.T) {
	src := "union\n  partner Margaret\n\nunion\n  partner Jane\n"
	nodes := parse(t, src)
	if len(nodes) != 2 {
		t.Fatalf("want 2 union nodes, got %d", len(nodes))
	}
	if nodes[0].Children[0].Line != "partner Margaret" {
		t.Errorf("got %q", nodes[0].Children[0].Line)
	}
	if nodes[1].Children[0].Line != "partner Jane" {
		t.Errorf("got %q", nodes[1].Children[0].Line)
	}
}

// ---- values ----

func TestValueWithSpecialChars(t *testing.T) {
	nodes := parse(t, "place Toledo, Lucas County, Ohio, USA\n")
	if nodes[0].Line != "place Toledo, Lucas County, Ohio, USA" {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestValueWithColon(t *testing.T) {
	nodes := parse(t, "note see also: church records\n")
	if nodes[0].Line != "note see also: church records" {
		t.Errorf("got %q", nodes[0].Line)
	}
}

// ---- quoted values ----

func TestQuotedValue(t *testing.T) {
	// Quotes are emitted as-is; the caller is responsible for stripping them.
	nodes := parse(t, `name "Margaret Louise Carter"`+"\n")
	if nodes[0].Line != `name "Margaret Louise Carter"` {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestQuotedValuePreservesInnerWhitespace(t *testing.T) {
	nodes := parse(t, `note "  leading space preserved  "`+"\n")
	if nodes[0].Line != `note "  leading space preserved  "` {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestEscapedQuoteInValue(t *testing.T) {
	nodes := parse(t, `name "he said \"hello\""`+"\n")
	if nodes[0].Line != `name "he said \"hello\""` {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestMixedValue(t *testing.T) {
	nodes := parse(t, "foo bar \"baz\" `qux`\n")
	want := "foo bar \"baz\" `qux`"
	if nodes[0].Line != want {
		t.Errorf("got %q, want %q", nodes[0].Line, want)
	}
}

// ---- multiline backtick values ----

func TestMultilineValue(t *testing.T) {
	// Backtick delimiters and all whitespace are emitted as-is.
	src := "text `first line,\n      second line,\n      third line.`\n"
	nodes := parse(t, src)
	want := "text `first line,\n      second line,\n      third line.`"
	if nodes[0].Line != want {
		t.Errorf("got %q, want %q", nodes[0].Line, want)
	}
}

func TestMultilineValueAsChild(t *testing.T) {
	src := "footnote ref\n  text `Ohio birth certificate,\n        file no. 1923.`\n"
	nodes := parse(t, src)
	child := nodes[0].Children[0]
	want := "text `Ohio birth certificate,\n        file no. 1923.`"
	if child.Line != want {
		t.Errorf("got %q, want %q", child.Line, want)
	}
}

func TestSingleLineBacktick(t *testing.T) {
	nodes := parse(t, "tag `value`\n")
	if nodes[0].Line != "tag `value`" {
		t.Errorf("got %q", nodes[0].Line)
	}
}

func TestMultilineValueFollowedBySibling(t *testing.T) {
	src := "a `line one\n   line two.`\nb\n"
	nodes := parse(t, src)
	if len(nodes) != 2 {
		t.Fatalf("want 2 top-level nodes, got %d", len(nodes))
	}
	if nodes[1].Line != "b" {
		t.Errorf("nodes[1].Line = %q", nodes[1].Line)
	}
}

// ---- Write ----

func TestWriteTagOnly(t *testing.T) {
	nodes := []Node{{Line: "birth"}}
	if got := write(t, nodes); got != "birth\n" {
		t.Errorf("got %q", got)
	}
}

func TestWriteTagWithValue(t *testing.T) {
	nodes := []Node{{Line: "name Edward Thomas Miller"}}
	if got := write(t, nodes); got != "name Edward Thomas Miller\n" {
		t.Errorf("got %q", got)
	}
}

func TestWriteChildren(t *testing.T) {
	nodes := []Node{{
		Line: "birth",
		Children: []Node{
			{Line: "date 1923-04-17"},
			{Line: "place Toledo, Ohio"},
		},
	}}
	want := "birth\n  date 1923-04-17\n  place Toledo, Ohio\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteDeepNesting(t *testing.T) {
	nodes := []Node{{
		Line: "person Alice",
		Children: []Node{{
			Line: "birth",
			Children: []Node{
				{Line: "date 1960-02-03"},
			},
		}},
	}}
	want := "person Alice\n  birth\n    date 1960-02-03\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteQuotedValue(t *testing.T) {
	// The caller encodes the value; Write emits it as-is.
	nodes := []Node{{Line: `note "  leading space  "`}}
	want := "note \"  leading space  \"\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteMultilineValue(t *testing.T) {
	// Caller provides the backtick-wrapped value; embedded newlines produce
	// multiline output naturally.
	nodes := []Node{{Line: "text `line one,\nline two,\nline three.`"}}
	want := "text `line one,\nline two,\nline three.`\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteMultilineValueAsChild(t *testing.T) {
	nodes := []Node{{
		Line: "footnote ref",
		Children: []Node{{
			Line: "text `Ohio birth certificate,\nfile no. 1923.`",
		}},
	}}
	want := "footnote ref\n  text `Ohio birth certificate,\nfile no. 1923.`\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteEmpty(t *testing.T) {
	if got := write(t, nil); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestWriteCommentNode(t *testing.T) {
	nodes := []Node{{Line: "# a comment"}, {Line: "birth"}}
	want := "# a comment\nbirth\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestRoundTrip parses the spec's example document and verifies that
// writing it back and re-parsing produces the same tree.
func TestRoundTrip(t *testing.T) {
	const doc = `schema 1
id 7K4M9Q2
name Edward Thomas Miller
birth
  date 1923-04-17
  place Toledo, Lucas County, Ohio, USA
death
  date 1998-11-02
  place Ann Arbor, Washtenaw County, Michigan, USA
union
  partner Margaret
  start-date 1946-06-08
footnote edward-birth-cert
  text ` + "`" + `Ohio birth certificate for Edward Thomas Miller,
        file no. 1923-0417-ETM, obtained from the Ohio
        Department of Health in 2002.` + "`" + `
`
	first := parse(t, doc)
	out := write(t, first)
	second := parse(t, out)
	assertNodesEqual(t, first, second, "")
}

func assertNodesEqual(t *testing.T, a, b []Node, path string) {
	t.Helper()
	if len(a) != len(b) {
		t.Errorf("%s: node count %d != %d", path, len(a), len(b))
		return
	}
	for i := range a {
		p := fmt.Sprintf("%s[%d]", path, i)
		if a[i].Line != b[i].Line {
			t.Errorf("%s: line %q != %q", p, a[i].Line, b[i].Line)
		}
		assertNodesEqual(t, a[i].Children, b[i].Children, p)
	}
}

// ---- TrimIndent / AddIndent ----

func TestTrimIndent(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// common indent stripped
		{"  foo\n  bar", "foo\nbar"},
		// partial indent: strip only the minimum
		{"    foo\n  bar", "  foo\nbar"},
		// blank lines excluded from measurement, preserved in output
		{"  foo\n\n  bar", "foo\n\nbar"},
		// whitespace-only lines excluded from measurement
		{"  foo\n   \n  bar", "foo\n \nbar"},
		// no common indent
		{"foo\nbar", "foo\nbar"},
		// zero common indent (first line at column 0)
		{"foo\n  bar", "foo\n  bar"},
		// single line
		{"  hello", "hello"},
		// empty string
		{"", ""},
	}
	for _, c := range cases {
		got := TrimIndent(c.in)
		if got != c.want {
			t.Errorf("TrimIndent(%q)\n  got  %q\n  want %q", c.in, got, c.want)
		}
	}
}

func TestAddIndent(t *testing.T) {
	cases := []struct {
		in     string
		indent string
		want   string
	}{
		{"foo\nbar", "  ", "  foo\n  bar"},
		// blank lines not indented
		{"foo\n\nbar", "  ", "  foo\n\n  bar"},
		// empty indent is a no-op
		{"foo\nbar", "", "foo\nbar"},
		// single line
		{"hello", "\t", "\thello"},
	}
	for _, c := range cases {
		got := AddIndent(c.in, c.indent)
		if got != c.want {
			t.Errorf("AddIndent(%q, %q)\n  got  %q\n  want %q", c.in, c.indent, got, c.want)
		}
	}
}

func TestTrimAddIndentRoundTrip(t *testing.T) {
	s := "  foo\n  bar\n\n  baz"
	trimmed := TrimIndent(s)
	if got := AddIndent(trimmed, "  "); got != s {
		t.Errorf("round-trip failed:\n  got  %q\n  want %q", got, s)
	}
}

// ---- NewNode, Head, Args, SetArgs ----

func TestNewNode(t *testing.T) {
	n := NewNode("birth")
	if n.Line != "birth" {
		t.Errorf("got %q", n.Line)
	}

	// multi-word arg gets quoted; Args() decodes it back
	n = NewNode("name", "Edward Thomas Miller")
	if n.Line != `name "Edward Thomas Miller"` {
		t.Errorf("got %q", n.Line)
	}
	if args := n.Args(); len(args) != 2 || args[1] != "Edward Thomas Miller" {
		t.Errorf("args = %v", args)
	}

	// args requiring quoting
	n = NewNode("note", `he said "hi"`)
	if n.Head() != "note" {
		t.Errorf("head = %q", n.Head())
	}
	if args := n.Args(); len(args) != 2 || args[1] != `he said "hi"` {
		t.Errorf("args = %v", args)
	}
}

func TestHead(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"birth", "birth"},
		{"date 1923-04-17", "date"},
		{"name Edward Thomas Miller", "name"},
		{"", ""},
	}
	for _, c := range cases {
		n := Node{Line: c.line}
		if got := n.Head(); got != c.want {
			t.Errorf("Head(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestArgs(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		// plain tokens
		{"date 1923-04-17", []string{"date", "1923-04-17"}},
		{"name Edward Thomas Miller", []string{"name", "Edward", "Thomas", "Miller"}},
		// double-quoted: Unquote decodes escapes
		{`name "Edward Thomas Miller"`, []string{"name", "Edward Thomas Miller"}},
		{`note "he said \"hi\""`, []string{"note", `he said "hi"`}},
		// backtick: delimiters stripped, content raw
		{"text `raw content here`", []string{"text", "raw content here"}},
		// mixed
		{"foo plain `raw` \"decoded\"", []string{"foo", "plain", "raw", "decoded"}},
		// empty
		{"", nil},
		// tag only
		{"birth", []string{"birth"}},
	}
	for _, c := range cases {
		n := Node{Line: c.line}
		got := n.Args()
		if len(got) != len(c.want) {
			t.Errorf("Args(%q): got %v, want %v", c.line, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("Args(%q)[%d] = %q, want %q", c.line, i, got[i], c.want[i])
			}
		}
	}
}

func TestArgsRoundTrip(t *testing.T) {
	// SetArgs + Args should round-trip for any slice of strings.
	cases := [][]string{
		{"birth"},
		{"name", "Edward Thomas Miller"},
		{"note", `he said "hi"`},
		{"text", "line one\nline two"},
		{"mixed", "plain", "with spaces", "back`tick"},
	}
	for _, args := range cases {
		var n Node
		n.SetArgs(args)
		got := n.Args()
		if len(got) != len(args) {
			t.Errorf("SetArgs(%v) → Args(): got %v", args, got)
			continue
		}
		for i := range got {
			if got[i] != args[i] {
				t.Errorf("SetArgs(%v) → Args()[%d] = %q, want %q", args, i, got[i], args[i])
			}
		}
	}
}

// ---- Quote ----

func TestQuote(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// no special chars: pass through
		{"hello", "hello"},
		{"birth-date", "birth-date"},
		// contains whitespace: strconv.Quote
		{"hello world", `"hello world"`},
		// contains double-quote, no backtick: wrap in backticks
		{`say "hi"`, "`say \"hi\"`"},
		// contains backtick, no double-quote: strconv.Quote
		{"back`tick", `"back` + "`" + `tick"`},
		// contains both: strconv.Quote
		{`say "hi" and ` + "`bye`", `"say \"hi\" and ` + "`" + `bye` + "`" + `"`},
	}
	for _, c := range cases {
		got := Quote(c.in)
		if got != c.want {
			t.Errorf("Quote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
