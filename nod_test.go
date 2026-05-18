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
	if nodes[0].Tag != "name" || nodes[0].Value != "" {
		t.Errorf("got tag=%q value=%q", nodes[0].Tag, nodes[0].Value)
	}
}

func TestTagWithValue(t *testing.T) {
	nodes := parse(t, "name Edward Thomas Miller\n")
	if nodes[0].Value != "Edward Thomas Miller" {
		t.Errorf("got %q", nodes[0].Value)
	}
}

func TestMultipleTopLevel(t *testing.T) {
	nodes := parse(t, "a\nb\nc\n")
	if len(nodes) != 3 {
		t.Fatalf("want 3 nodes, got %d", len(nodes))
	}
	for i, tag := range []string{"a", "b", "c"} {
		if nodes[i].Tag != tag {
			t.Errorf("nodes[%d].Tag = %q, want %q", i, nodes[i].Tag, tag)
		}
	}
}

func TestEmpty(t *testing.T) {
	nodes := parse(t, "")
	if len(nodes) != 0 {
		t.Fatalf("want 0 nodes, got %d", len(nodes))
	}
}

// ---- blank lines and comments ----

func TestBlankLinesIgnored(t *testing.T) {
	nodes := parse(t, "\n\na\n\nb\n\n")
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
}

func TestCommentLinesIgnored(t *testing.T) {
	nodes := parse(t, "# comment\na\n# another\nb\n")
	if len(nodes) != 2 {
		t.Fatalf("want 2, got %d", len(nodes))
	}
}

func TestInlineHashIsLiteral(t *testing.T) {
	nodes := parse(t, "note see also: church records # not a comment\n")
	want := "see also: church records # not a comment"
	if nodes[0].Value != want {
		t.Errorf("got %q, want %q", nodes[0].Value, want)
	}
}

// ---- nesting ----

func TestChildren(t *testing.T) {
	nodes := parse(t, "birth\n  date 1923-04-17\n  place Toledo\n")
	if len(nodes) != 1 {
		t.Fatalf("want 1 top-level node, got %d", len(nodes))
	}
	birth := nodes[0]
	if birth.Tag != "birth" {
		t.Fatalf("tag = %q", birth.Tag)
	}
	if len(birth.Children) != 2 {
		t.Fatalf("want 2 children, got %d", len(birth.Children))
	}
	if birth.Children[0].Tag != "date" || birth.Children[0].Value != "1923-04-17" {
		t.Errorf("child[0]: %+v", birth.Children[0])
	}
	if birth.Children[1].Tag != "place" || birth.Children[1].Value != "Toledo" {
		t.Errorf("child[1]: %+v", birth.Children[1])
	}
}

func TestDeepNesting(t *testing.T) {
	src := "person Alice\n  birth\n    date 1960-02-03\n    place Chicago\n"
	nodes := parse(t, src)
	if len(nodes) != 1 {
		t.Fatalf("want 1 top-level, got %d", len(nodes))
	}
	birth := nodes[0].Children[0]
	if birth.Tag != "birth" || len(birth.Children) != 2 {
		t.Errorf("unexpected birth node: %+v", birth)
	}
}

func TestNodeWithValueAndChildren(t *testing.T) {
	src := "footnote edward-birth-cert\n  text Ohio birth certificate.\n"
	nodes := parse(t, src)
	fn := nodes[0]
	if fn.Value != "edward-birth-cert" {
		t.Errorf("value = %q", fn.Value)
	}
	if len(fn.Children) != 1 || fn.Children[0].Tag != "text" {
		t.Errorf("children = %+v", fn.Children)
	}
}

func TestSiblingsAfterChildren(t *testing.T) {
	src := "a\n  b\nc\n"
	nodes := parse(t, src)
	if len(nodes) != 2 {
		t.Fatalf("want 2 top-level nodes, got %d", len(nodes))
	}
	if nodes[1].Tag != "c" {
		t.Errorf("nodes[1].Tag = %q", nodes[1].Tag)
	}
}

func TestNonUniformIndent(t *testing.T) {
	// Spec allows any consistent indentation depth per level
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
	if len(b.Children) != 1 || b.Children[0].Tag != "c" {
		t.Errorf("b.Children = %+v", b.Children)
	}
	if a.Children[1].Tag != "d" {
		t.Errorf("a.Children[1].Tag = %q", a.Children[1].Tag)
	}
}

// ---- implicit lists ----

func TestRepeatedTags(t *testing.T) {
	src := "union\n  partner Margaret\n\nunion\n  partner Jane\n"
	nodes := parse(t, src)
	if len(nodes) != 2 {
		t.Fatalf("want 2 union nodes, got %d", len(nodes))
	}
	if nodes[0].Children[0].Value != "Margaret" {
		t.Errorf("got %q", nodes[0].Children[0].Value)
	}
	if nodes[1].Children[0].Value != "Jane" {
		t.Errorf("got %q", nodes[1].Children[0].Value)
	}
}

// ---- values ----

func TestValueWithSpecialChars(t *testing.T) {
	nodes := parse(t, "place Toledo, Lucas County, Ohio, USA\n")
	if nodes[0].Value != "Toledo, Lucas County, Ohio, USA" {
		t.Errorf("got %q", nodes[0].Value)
	}
}

func TestValueWithColon(t *testing.T) {
	nodes := parse(t, "note see also: church records\n")
	if nodes[0].Value != "see also: church records" {
		t.Errorf("got %q", nodes[0].Value)
	}
}

// ---- quoted values ----

func TestQuotedValue(t *testing.T) {
	nodes := parse(t, `name "Margaret Louise Carter"`+"\n")
	if nodes[0].Value != "Margaret Louise Carter" {
		t.Errorf("got %q", nodes[0].Value)
	}
}

func TestQuotedValuePreservesInnerWhitespace(t *testing.T) {
	nodes := parse(t, `note "  leading space preserved  "`+"\n")
	if nodes[0].Value != "  leading space preserved  " {
		t.Errorf("got %q", nodes[0].Value)
	}
}

// ---- multiline backtick values ----

func TestMultilineValue(t *testing.T) {
	src := "text `first line,\n      second line,\n      third line.`\n"
	nodes := parse(t, src)
	want := "first line,\nsecond line,\nthird line."
	if nodes[0].Value != want {
		t.Errorf("got %q, want %q", nodes[0].Value, want)
	}
}

func TestMultilineValueAsChild(t *testing.T) {
	src := "footnote ref\n  text `Ohio birth certificate,\n        file no. 1923.`\n"
	nodes := parse(t, src)
	child := nodes[0].Children[0]
	want := "Ohio birth certificate,\nfile no. 1923."
	if child.Value != want {
		t.Errorf("got %q, want %q", child.Value, want)
	}
}

func TestSingleLineBacktick(t *testing.T) {
	nodes := parse(t, "tag `value`\n")
	if nodes[0].Value != "value" {
		t.Errorf("got %q", nodes[0].Value)
	}
}

func TestMultilineValueFollowedBySibling(t *testing.T) {
	src := "a `line one\n   line two.`\nb\n"
	nodes := parse(t, src)
	if len(nodes) != 2 {
		t.Fatalf("want 2 top-level nodes, got %d", len(nodes))
	}
	if nodes[1].Tag != "b" {
		t.Errorf("nodes[1].Tag = %q", nodes[1].Tag)
	}
}

// ---- line numbers ----

func TestLineNumbers(t *testing.T) {
	src := "# comment\n\nschema 1\nname Edward\n\nbirth\n  date 1923-04-17\n"
	nodes := parse(t, src)

	cases := []struct {
		tag  string
		line int
	}{
		{"schema", 3},
		{"name", 4},
		{"birth", 6},
	}
	for i, c := range cases {
		if nodes[i].Tag != c.tag {
			t.Errorf("[%d] tag = %q, want %q", i, nodes[i].Tag, c.tag)
		}
		if nodes[i].Line != c.line {
			t.Errorf("[%d] %q: line = %d, want %d", i, c.tag, nodes[i].Line, c.line)
		}
	}

	date := nodes[2].Children[0]
	if date.Line != 7 {
		t.Errorf("date line = %d, want 7", date.Line)
	}
}

func TestLineNumberMultilineValue(t *testing.T) {
	// Node line should point to the opening tag line, not the closing backtick
	src := "a `line one\n   line two.`\nb\n"
	nodes := parse(t, src)
	if nodes[0].Line != 1 {
		t.Errorf("a.Line = %d, want 1", nodes[0].Line)
	}
	if nodes[1].Line != 3 {
		t.Errorf("b.Line = %d, want 3", nodes[1].Line)
	}
}

// ---- Write ----

func TestWriteTagOnly(t *testing.T) {
	nodes := []Node{{Tag: "birth"}}
	if got := write(t, nodes); got != "birth\n" {
		t.Errorf("got %q", got)
	}
}

func TestWriteTagWithValue(t *testing.T) {
	nodes := []Node{{Tag: "name", Value: "Edward Thomas Miller"}}
	if got := write(t, nodes); got != "name Edward Thomas Miller\n" {
		t.Errorf("got %q", got)
	}
}

func TestWriteChildren(t *testing.T) {
	nodes := []Node{{
		Tag: "birth",
		Children: []Node{
			{Tag: "date", Value: "1923-04-17"},
			{Tag: "place", Value: "Toledo, Ohio"},
		},
	}}
	want := "birth\n  date 1923-04-17\n  place Toledo, Ohio\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteDeepNesting(t *testing.T) {
	nodes := []Node{{
		Tag:   "person",
		Value: "Alice",
		Children: []Node{{
			Tag: "birth",
			Children: []Node{
				{Tag: "date", Value: "1960-02-03"},
			},
		}},
	}}
	want := "person Alice\n  birth\n    date 1960-02-03\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteQuotedValue(t *testing.T) {
	nodes := []Node{{Tag: "note", Value: "  leading space  "}}
	want := "note \"  leading space  \"\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteMultilineValue(t *testing.T) {
	nodes := []Node{{Tag: "text", Value: "line one,\nline two,\nline three."}}
	want := "text `line one,\n      line two,\n      line three.`\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteMultilineValueAsChild(t *testing.T) {
	nodes := []Node{{
		Tag:   "footnote",
		Value: "ref",
		Children: []Node{{
			Tag:   "text",
			Value: "Ohio birth certificate,\nfile no. 1923.",
		}},
	}}
	want := "footnote ref\n  text `Ohio birth certificate,\n        file no. 1923.`\n"
	if got := write(t, nodes); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteEmpty(t *testing.T) {
	if got := write(t, nil); got != "" {
		t.Errorf("got %q", got)
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
		p := fmt.Sprintf("%s[%d](%s)", path, i, a[i].Tag)
		if a[i].Tag != b[i].Tag {
			t.Errorf("%s: tag %q != %q", p, a[i].Tag, b[i].Tag)
		}
		if a[i].Value != b[i].Value {
			t.Errorf("%s: value %q != %q", p, a[i].Value, b[i].Value)
		}
		assertNodesEqual(t, a[i].Children, b[i].Children, p)
	}
}
