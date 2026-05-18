# nod
is a human-writable data format built on the simplest possible node data structure

# Nod format spec

- Files are UTF-8, extension .nod
- Indentation is spaces only, any consistent depth
- Comments start with #
- One token on a line → block node (has children)
- Two+ tokens on a line → leaf node (tag + value)
- Value is everything after the first token, trimmed
- Backtick strings span multiple lines, leading whitespace stripped
- Quotes are optional, honored when present
- Repeat tags are valid — implicit list, no special syntax
- All values are strings. Type interpretation is the caller's problem.

- 
