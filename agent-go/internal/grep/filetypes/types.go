package filetypes

import "path/filepath"

// Extensions returns the file extensions for a given type name.
// Type names are compatible with ripgrep's --type flag.
func Extensions(typeName string) []string {
	return typeMap[typeName]
}

// MatchesType returns true if the file path has an extension matching the type.
func MatchesType(path, typeName string) bool {
	exts := Extensions(typeName)
	if exts == nil {
		return false
	}
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	for _, e := range exts {
		// Some "extensions" are full filenames (e.g. "Dockerfile", "Makefile")
		if e[0] != '.' {
			if base == e {
				return true
			}
		} else if ext == e {
			return true
		}
	}
	return false
}

var typeMap = map[string][]string{
	"go":         {".go"},
	"js":         {".js", ".jsx", ".mjs", ".cjs"},
	"ts":         {".ts", ".tsx", ".mts", ".cts"},
	"py":         {".py", ".pyi", ".pyw"},
	"rust":       {".rs"},
	"java":       {".java"},
	"c":          {".c", ".h"},
	"cpp":        {".cpp", ".cc", ".cxx", ".hpp", ".hxx", ".h"},
	"cs":         {".cs"},
	"rb":         {".rb", ".erb"},
	"php":        {".php", ".php3", ".php4", ".php5", ".phtml"},
	"swift":      {".swift"},
	"kotlin":     {".kt", ".kts"},
	"scala":      {".scala", ".sc"},
	"html":       {".html", ".htm", ".xhtml"},
	"css":        {".css", ".scss", ".sass", ".less"},
	"json":       {".json", ".jsonl", ".geojson"},
	"yaml":       {".yaml", ".yml"},
	"toml":       {".toml"},
	"xml":        {".xml", ".xsl", ".xslt", ".svg"},
	"md":         {".md", ".markdown", ".mkd"},
	"sh":         {".sh", ".bash", ".zsh", ".fish"},
	"sql":        {".sql"},
	"graphql":    {".graphql", ".gql"},
	"proto":      {".proto"},
	"dockerfile": {"Dockerfile"},
	"make":       {"Makefile", "makefile", "GNUmakefile", ".mk"},
	"lua":        {".lua"},
	"r":          {".r", ".R", ".Rmd"},
	"dart":       {".dart"},
	"elixir":     {".ex", ".exs"},
	"erlang":     {".erl", ".hrl"},
	"haskell":    {".hs", ".lhs"},
	"ocaml":      {".ml", ".mli"},
	"zig":        {".zig"},
	"nim":        {".nim"},
	"vue":        {".vue"},
	"svelte":     {".svelte"},
}
