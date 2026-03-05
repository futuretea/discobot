package literal

// Seq represents a literal byte sequence extracted from a regex.
type Seq struct {
	Bytes []byte
	Exact bool // true if this literal is the complete match (no regex verification needed)
}

// Literals holds the result of literal extraction from a regex AST.
type Literals struct {
	Seqs     []Seq
	Position string // "prefix" or "none"
}

// IsEmpty returns true if no useful literals were extracted.
func (l *Literals) IsEmpty() bool {
	return len(l.Seqs) == 0 || l.Position == "none"
}
